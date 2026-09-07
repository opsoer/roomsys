// seedmedia 通过 HTTP API 为已有演示数据补充图片：
//   - 每个公寓 1 张封面（POST /api/building/cover）
//   - 每个房间 1 张照片（POST /api/building/rooms/:id/media）
//
// 使用场景：跑完 scripts/seed 后公寓/房间没有图片，前端列表与详情页观感差；
// 本脚本按公寓 id / 房间 id 作稳定 seed 从 picsum.photos 拉取随机图片，
// 同一 id 永远得到同一张图，可重复运行（已存在的下载缓存与上传结果不重复处理）。
//
// 执行流程（main 三阶段）：
//  1. root/root 登录，GET /api/admin/buildings 拉全部公寓；逐个公寓用「公寓 id」账号登录，
//     GET /api/building/rooms 拉房间列表（注意：单页最多 100 条，每公寓超过 100 间需自行改分页）
//  2. 阶段一：16 并发从 picsum.photos 下载图片到本地缓存目录（封面 1200x800 / 房间图 800x600）
//  3. 阶段二：8 并发上传，每个公寓先登录拿 token 再依次传封面 + 房间图
//
// 限流说明：服务端有全局 IP 限流（默认 240 次/分钟，见 middleware/ratelimit.go），
// 数千次上传必触发 429。脚本默认自限速 200 次/分钟（仅约束对后端的请求，
// 下载图片不受影响），收到 429 自动等限流窗口重置后重试。
//
// 环境变量：
//   SEED_BASE_URL  后端地址，默认 http://127.0.0.1:8081
//   SEED_TMP_DIR   图片下载缓存目录，默认 <系统临时目录>/seedmedia
//   SEED_RATE_PER_MIN  脚本自限速（次/分钟），默认 200；后端已调大 RATE_LIMIT_PER_MIN 时
//                      可设 0 关闭以提速
//
// 前置条件：
//  1. 后端已运行，且已通过 ./scripts/seed 生成公寓/房间数据
//  2. 楼栋管理员账号存在（用户名与密码均为公寓 id，如公寓 3 → 3 / 3）
//  3. 机器能访问 picsum.photos（外网）
//
// 运行方式（在 server 目录下）：go run ./scripts/seedmedia
// 预计耗时：5000+ 张图在默认自限速下约 25~30 分钟（大头是上传）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

const defaultBaseURL = "http://127.0.0.1:8081"

var baseURL = os.Getenv("SEED_BASE_URL")

var tmpDir = os.Getenv("SEED_TMP_DIR")

// reqLimiter 全局请求节流器，约束对后端的请求频率，避免触发服务端每 IP 限流。
// 下载/上传在多个 goroutine 间并发，靠共享的时间片排队实现全局限速。
var reqLimiter = newThrottle(envInt("SEED_RATE_PER_MIN", 200))

// envInt 读取整型环境变量，未设置或非法时返回默认值
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := fmt.Sscanf(v, "%d", &def); n == 1 && err == nil {
			return def
		}
	}
	return def
}

// throttle 串行化所有请求的发起时刻，保证请求频率不超过 perMinute 次/分钟。
// 原理：记录下一次允许请求的时间点，每个请求先排队等到自己的时间片。
func newThrottle(perMinute int) *throttle {
	if perMinute <= 0 {
		return &throttle{} // 0 或负数 = 关闭自限速
	}
	return &throttle{minInterval: time.Minute / time.Duration(perMinute)}
}

type throttle struct {
	mu          sync.Mutex
	next        time.Time
	minInterval time.Duration
}

func (t *throttle) wait() {
	if t.minInterval <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if now := time.Now(); t.next.After(now) {
		time.Sleep(t.next.Sub(now))
	}
	t.next = time.Now().Add(t.minInterval)
}

const (
	downloadWorkers = 16
	uploadWorkers   = 8
	roomImageW      = 800
	roomImageH      = 600
	coverW          = 1200
	coverH          = 800
)

// apiResp 与后端统一的响应结构
type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type apiError struct {
	Status  int
	Code    int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("HTTP %d (code %d): %s", e.Status, e.Code, e.Message)
}

// doJSON 发送 JSON 请求
func doJSON(method, path, token string, body interface{}, out interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	return doReq(method, path, token, "application/json", bodyBytes, out)
}

// doUpload 发送 multipart 文件上传请求
func doUpload(path, token, field, filename string, data []byte, extra map[string]string, out interface{}) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	for k, v := range extra {
		if err := writer.WriteField(k, v); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return doReq("POST", path, token, writer.FormDataContentType(), body.Bytes(), out)
}

// doReq 发送请求，自动重试三类失败：网络错误、5xx、429 限流；
// 其余 4xx（参数/权限/冲突等）直接返回错误不重试。
func doReq(method, path, token, contentType string, bodyBytes []byte, out interface{}) error {
	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err := doOnce(method, path, token, contentType, bodyBytes, out); err != nil {
			lastErr = err
			if ae, ok := err.(*apiError); ok {
				if ae.Status == http.StatusTooManyRequests {
					// 撞上服务端限流：限流窗口为 1 分钟，等窗口重置后重试
					time.Sleep(65 * time.Second)
					continue
				}
				if ae.Status < 500 {
					return err
				}
			}
			continue
		}
		return nil
	}
	return lastErr
}

func doOnce(method, path, token, contentType string, bodyBytes []byte, out interface{}) error {
	reqLimiter.wait()
	req, err := http.NewRequest(method, baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("解析响应失败: %v", err)
	}
	if resp.StatusCode >= 400 || r.Code != 0 {
		return &apiError{Status: resp.StatusCode, Code: r.Code, Message: r.Message}
	}
	if out != nil && len(r.Data) > 0 {
		return json.Unmarshal(r.Data, out)
	}
	return nil
}

func login(username, password string) (string, error) {
	var data struct {
		Token string `json:"token"`
	}
	if err := doJSON("POST", "/api/auth/login", "", map[string]string{"username": username, "password": password}, &data); err != nil {
		return "", err
	}
	return data.Token, nil
}

// download 下载图片到临时目录，返回本地路径
func download(seed string, w, h int, outDir string) (string, error) {
	path := filepath.Join(outDir, seed+".jpg")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	url := fmt.Sprintf("https://picsum.photos/seed/%s/%d/%d", seed, w, h)

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 800 * time.Millisecond)
		}
		resp, err := http.Get(url)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || len(data) < 1024 {
			if err != nil {
				lastErr = err
			}
			continue
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return "", err
		}
		return path, nil
	}
	return "", lastErr
}

type building struct {
	ID   uint
	Rooms []uint
}

func main() {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if tmpDir == "" {
		tmpDir = filepath.Join(os.TempDir(), "seedmedia")
	}
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		panic(err)
	}

	rootToken, err := login("root", "root")
	if err != nil {
		panic(fmt.Sprintf("超级管理员登录失败：%v", err))
	}

	// 获取全部公寓与房间
	var listData struct {
		Buildings []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"buildings"`
	}
	if err := doJSON("GET", "/api/admin/buildings?page=1&page_size=100", rootToken, nil, &listData); err != nil {
		panic(fmt.Sprintf("获取公寓列表失败：%v", err))
	}

	var buildings []building
	for _, b := range listData.Buildings {
		username := fmt.Sprintf("%d", b.ID)
		token, err := login(username, username)
		if err != nil {
			panic(fmt.Sprintf("楼栋管理员 %s 登录失败：%v", username, err))
		}
		var rooms struct {
			Rooms []struct {
				ID uint `json:"id"`
			} `json:"rooms"`
		}
		if err := doJSON("GET", "/api/building/rooms?page=1&page_size=100", token, nil, &rooms); err != nil {
			panic(fmt.Sprintf("获取公寓 %d 房间列表失败：%v", b.ID, err))
		}
		ids := make([]uint, 0, len(rooms.Rooms))
		for _, r := range rooms.Rooms {
			ids = append(ids, r.ID)
		}
		buildings = append(buildings, building{ID: b.ID, Rooms: ids})
	}
	fmt.Printf("共 %d 个公寓，%d 个房间\n", len(buildings), totalRooms(buildings))

	// 阶段一：下载图片（封面 + 房间图）
	start := time.Now()
	downloadImages(buildings, tmpDir)
	fmt.Printf("图片下载完成，耗时 %s\n", time.Since(start).Round(time.Second))

	// 阶段二：上传图片
	start = time.Now()
	var uploaded int64
	uploadImages(buildings, tmpDir, &uploaded)
	fmt.Printf("上传完成：%d 张，耗时 %s\n", uploaded, time.Since(start).Round(time.Second))
}

func totalRooms(bs []building) int {
	n := 0
	for _, b := range bs {
		n += len(b.Rooms)
	}
	return n
}

// downloadImages 并行下载全部图片
func downloadImages(buildings []building, outDir string) {
	type task struct {
		seed string
		w, h int
	}
	var tasks []task
	for _, b := range buildings {
		tasks = append(tasks, task{seed: fmt.Sprintf("b%03d", b.ID), w: coverW, h: coverH})
		for _, rid := range b.Rooms {
			tasks = append(tasks, task{seed: fmt.Sprintf("r%06d", rid), w: roomImageW, h: roomImageH})
		}
	}

	var wg sync.WaitGroup
	var counter int64
	taskCh := make(chan task)
	for i := 0; i < downloadWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range taskCh {
				if _, err := download(t.seed, t.w, t.h, outDir); err != nil {
					fmt.Printf("下载失败 %s: %v\n", t.seed, err)
					continue
				}
				n := atomic.AddInt64(&counter, 1)
				if n%500 == 0 {
					fmt.Printf("已下载 %d/%d\n", n, len(tasks))
				}
			}
		}()
	}
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)
	wg.Wait()
	fmt.Printf("已下载 %d/%d\n", counter, len(tasks))
}

// uploadImages 并行上传封面与房间图片（每个公寓登录一次）
func uploadImages(buildings []building, outDir string, uploaded *int64) {
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		tokens  = make(map[uint]string)
		failCnt int64
	)
	taskCh := make(chan func())
	for i := 0; i < uploadWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fn := range taskCh {
				fn()
			}
		}()
	}

	for _, b := range buildings {
		b := b
		taskCh <- func() {
			mu.Lock()
			token, ok := tokens[b.ID]
			if !ok {
				var err error
				username := fmt.Sprintf("%d", b.ID)
				token, err = login(username, username)
				if err != nil {
					mu.Unlock()
					fmt.Printf("楼栋管理员 %s 登录失败：%v\n", username, err)
					return
				}
				tokens[b.ID] = token
			}
			mu.Unlock()

			// 封面
			coverPath := filepath.Join(outDir, fmt.Sprintf("b%03d", b.ID)+".jpg")
			if err := uploadCover(b.ID, token, coverPath); err != nil {
				fmt.Printf("公寓 %d 封面上传失败：%v\n", b.ID, err)
			} else {
				atomic.AddInt64(uploaded, 1)
			}

			// 房间图片
			for _, rid := range b.Rooms {
				imgPath := filepath.Join(outDir, fmt.Sprintf("r%06d", rid)+".jpg")
				if err := uploadRoomImage(b.ID, rid, token, imgPath); err != nil {
					n := atomic.AddInt64(&failCnt, 1)
					if n <= 10 {
						fmt.Printf("房间 %d 图片上传失败：%v\n", rid, err)
					}
					continue
				}
				n := atomic.AddInt64(uploaded, 1)
				if n%500 == 0 {
					fmt.Printf("已上传 %d 张\n", n)
				}
			}
		}
	}
	close(taskCh)
	wg.Wait()
	if failCnt > 0 {
		fmt.Printf("共 %d 张上传失败\n", failCnt)
	}
}

func uploadCover(buildingID uint, token, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return doUpload("/api/building/cover", token, "file", fmt.Sprintf("cover_%d.jpg", buildingID), data, nil, nil)
}

func uploadRoomImage(buildingID, roomID uint, token, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return doUpload(
		fmt.Sprintf("/api/building/rooms/%d/media", roomID),
		token, "file", fmt.Sprintf("room_%d.jpg", roomID), data,
		map[string]string{"category": "gallery"}, nil,
	)
}