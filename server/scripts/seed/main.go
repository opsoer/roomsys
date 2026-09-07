// seed 通过 HTTP API 生成演示数据：100 个公寓（含房东信息）+ 每栋 50 套房 + 约 70% 房间签约出租。
// 所有操作（创建公寓、创建楼栋管理员、登录、创建房间、出租签约）全部走真实 HTTP 接口，
// 因此数据与手工在管理后台录入完全一致（含账单、待办等派生数据）。
//
// 使用场景：
//   - 本地/演示环境快速灌入大批量逼真数据，验证列表分页、地图筛选、统计图表等在真实数据量下的表现
//   - 演示环境给客户看效果（配合 scripts/seedmedia 补充图片）
//   注意：会写入约 100 公寓 + 5000 房间 + 数千账单，只可用于可丢弃的数据库，严禁对生产库运行。
//
// 执行流程（main → seedBuildings → seedRooms）：
//  1. root/root 登录超级管理员，拿 JWT
//  2. 循环随机生成 100 个公寓（深圳各区/街道/小区名去重），POST /api/admin/buildings
//     （全套餐 full，contract_date=当天，到期日由后端按一年自动算出）
//  3. GET /api/admin/buildings 拉回全部公寓（不依赖自增 ID 连续）
//  4. 每个公寓：POST /api/admin/auth/create-building-admin 建管理员（用户名=密码=公寓 id），
//     再以该管理员身份登录
//  5. 每公寓创建 50 套房（25 单间 / 13 一室一厅 / 12 两房，随机楼层与房号），POST /api/building/rooms
//  6. 每公寓按 70% 概率随机选房，PUT /api/building/rooms/:id/status 设为 rented，
//     由后端自动生成租客、合同与账单
//
// 限流说明：服务端有全局 IP 限流（默认 240 次/分钟，见 middleware/ratelimit.go），
// 本脚本全速跑约 9000 个请求必触发 429。因此脚本默认自限速 200 次/分钟，
// 且收到 429 后自动等待限流窗口重置再重试，无需人工干预。
//
// 环境变量：
//   SEED_BASE_URL      后端地址，默认 http://127.0.0.1:8081（与 config.json 的 server_port 对应）
//   SEED_RATE_PER_MIN  脚本自限速（次/分钟），默认 200；若启动后端时已调大
//                      RATE_LIMIT_PER_MIN，可设为 0 关闭自限速以提速
//
// 默认约定：
//   - 每个公寓创建 1 个管理账号，用户名与密码均为公寓 id（如公寓 3 → 3 / 3）
//   - 公寓一律全套餐（full），签约时间（contract_date）为跑脚本当天，到期时间自动为一年后
//
// 前置条件：
//  1. 后端服务已在 SEED_BASE_URL 运行，且数据库已清空（脚本要求公宓名单恰好为空后再建 100 个）
//  2. 数据库中已有超级管理员 root/root（后端每次启动都会把 root 密码重置为 root）
//
// 运行方式（在 server 目录下）：go run ./scripts/seed
// 预计耗时：默认自限速下约 45 分钟（约 9000 个请求）；关闭自限速并调大服务端限流后约 3~5 分钟。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

const defaultBaseURL = "http://127.0.0.1:8081"

var baseURL = os.Getenv("SEED_BASE_URL")

// reqLimiter 全局请求节流器，避免触发服务端每 IP 限流（默认 240 次/分钟）
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

// 每栋楼房间数量与户型比例：一半单间、四分之一一室一厅、四分之一两房
const (
	roomsPerBuilding = 50
	studioCount      = 25
	oneBedCount      = 13
	twoBedCount      = 12
	rentProbability  = 0.7
)

// apiResp 与后端统一的响应结构
type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// apiError HTTP/业务错误
type apiError struct {
	Status  int
	Code    int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("HTTP %d (code %d): %s", e.Status, e.Code, e.Message)
}

// do 发送请求并解析统一响应。自动重试三类失败：网络错误、5xx、429 限流；
// 其余 4xx（参数/权限/冲突等）直接返回错误不重试。
func do(method, path, token string, body interface{}, out interface{}) error {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}

	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err := doOnce(method, path, token, bodyBytes, out); err != nil {
			lastErr = err
			ae, ok := err.(*apiError)
			if ok && ae.Status == http.StatusTooManyRequests {
				// 撞上服务端限流：限流窗口为 1 分钟，等窗口重置后重试
				time.Sleep(65 * time.Second)
				continue
			}
			if ok && ae.Status < 500 {
				return err
			}
			continue
		}
		return nil
	}
	return lastErr
}

func doOnce(method, path, token string, bodyBytes []byte, out interface{}) error {
	reqLimiter.wait()
	req, err := http.NewRequest(method, baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
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

// login 登录并返回访问令牌
func login(username, password string) (string, error) {
	var data struct {
		Token string `json:"token"`
	}
	if err := do("POST", "/api/auth/login", "", map[string]string{"username": username, "password": password}, &data); err != nil {
		return "", err
	}
	return data.Token, nil
}

type shenzhenDistrict struct {
	Name    string
	Streets []string
}

var districts = []shenzhenDistrict{
	{Name: "福田区", Streets: []string{"华强北街道", "梅林街道", "香蜜湖街道", "车公庙", "福田保税区", "竹子林", "岗厦", "下梅林"}},
	{Name: "南山区", Streets: []string{"科技园", "后海", "前海", "蛇口", "西丽", "桃源", "白石洲", "深圳湾"}},
	{Name: "罗湖区", Streets: []string{"东门", "笋岗", "翠竹", "莲塘", "黄贝岭", "水贝", "泥岗", "布心"}},
	{Name: "宝安区", Streets: []string{"西乡", "新安", "福永", "沙井", "松岗", "石岩", "航城", "福海"}},
	{Name: "龙岗区", Streets: []string{"坂田", "布吉", "龙城", "横岗", "南湾", "平湖", "坪地", "宝龙"}},
	{Name: "龙华区", Streets: []string{"民治", "大浪", "观澜", "观湖", "福城", "梅龙"}},
	{Name: "盐田区", Streets: []string{"沙头角", "海山", "梅沙", "盐田港"}},
	{Name: "光明区", Streets: []string{"光明", "公明", "新湖", "凤凰", "马田"}},
	{Name: "坪山区", Streets: []string{"坪山", "坑梓", "龙田", "马峦"}},
	{Name: "大鹏新区", Streets: []string{"大鹏", "葵涌", "南澳"}},
}

var villagePrefixes = []string{
	"阳光", "翠湖", "金鹏", "燕山", "锦绣", "碧海", "蓝湾", "云顶", "星河", "梧桐",
	"桂花", "荔枝", "梅园", "椰风", "海韵", "凤凰", "龙腾", "紫荆", "兰花", "松柏",
}
var villageSuffixes = []string{"花园", "家园", "名苑", "雅苑", "豪庭"}

var orientations = []string{"南", "北", "东", "西", "东南", "西南", "南北", "东西"}

var surnames = []string{
	"张", "王", "李", "赵", "刘", "陈", "杨", "黄", "周", "吴", "徐", "孙", "胡", "朱", "高", "林",
	"何", "郭", "马", "罗", "梁", "宋", "郑", "谢", "韩", "唐", "冯", "于", "董", "萧", "程", "曹",
	"袁", "邓", "许", "傅", "沈", "曾", "彭", "吕", "苏", "卢", "蒋", "蔡", "贾", "丁", "魏", "薛",
}
var givenNames = []string{
	"伟", "芳", "娜", "敏", "静", "磊", "军", "洋", "勇", "艳", "杰", "涛", "明", "超", "霞", "平",
	"刚", "建华", "文", "辉", "力", "鹏", "玉华", "海燕", "志强", "国强", "晓东", "宇", "诚", "浩", "光", "婷",
	"雪", "鑫", "欣", "怡", "雨", "睿", "晨", "阳", "佳", "琪", "淑华", "丽华", "美玲", "秀兰",
}

var now = time.Now()
var usedPhone = make(map[string]bool)

func main() {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	// 1. 超级管理员登录
	fmt.Printf("登录超级管理员 root ...\n")
	rootToken, err := login("root", "root")
	if err != nil {
		panic(fmt.Sprintf("超级管理员登录失败（请先启动后端并确认 root/root 可登录）：%v", err))
	}
	fmt.Println("超级管理员登录成功")

	seedBuildings(rootToken)
	seedRooms(rootToken)
	fmt.Println("\n全部完成！楼栋管理员账号：用户名=公寓id，密码同用户名（如公寓 3 → 3 / 3）")
}

// seedBuildings 创建 100 个公寓（全套餐、含房东信息、签约时间为跑脚本当天、一年后到期）
func seedBuildings(rootToken string) {
	usedAddr := make(map[string]bool)
	usedName := make(map[string]bool)

	buildings := 0
	for buildings < 100 {
		d := districts[rng.Intn(len(districts))]
		street := d.Streets[rng.Intn(len(d.Streets))]
		village := villagePrefixes[rng.Intn(len(villagePrefixes))] + villageSuffixes[rng.Intn(len(villageSuffixes))]
		buildingNo := fmt.Sprintf("%d栋", 1+rng.Intn(20))

		if usedAddr[d.Name+street+village+buildingNo] {
			continue
		}
		name := village + buildingNo
		if usedName[name] {
			continue
		}
		usedAddr[d.Name+street+village+buildingNo] = true
		usedName[name] = true

		landlordCount := 1 + rng.Intn(2)
		landlords := make([]map[string]string, 0, landlordCount)
		for i := 0; i < landlordCount; i++ {
			landlords = append(landlords, map[string]string{
				"name":  randomName(),
				"phone": randomPhone(),
			})
		}

		var data struct {
			Building struct {
				ID uint `json:"id"`
			} `json:"building"`
		}

		reqBody := map[string]interface{}{
			"name":          name,
			"package":       "full",
			"contract_date": now.Format("2006-01-02"), // 签约时间=跑脚本当天，到期=一年后（后端自动计算）
			"district":      d.Name,
			"street":        street,
			"village":       village,
			"building_no":   buildingNo,
			"description":   "精装修公寓，拎包入住，周边配套齐全，交通便利，直连房东无中介费。",
			"landlords":     landlords,
		}
		if err := do("POST", "/api/admin/buildings", rootToken, reqBody, &data); err != nil {
			panic(fmt.Sprintf("创建公寓 %s 失败：%v", name, err))
		}
		buildings++
		fmt.Printf("已创建公寓 %d/100：%s（id=%d，房东%d人）\n", buildings, name, data.Building.ID, landlordCount)
	}
}

// seedRooms 为每个公寓创建 50 套房，并随机出租其中约 70%
func seedRooms(rootToken string) {
	// 通过接口获取全部公寓，避免依赖自增 ID 连续
	var listData struct {
		Buildings []struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"buildings"`
		Total int `json:"total"`
	}
	if err := do("GET", "/api/admin/buildings?page=1&page_size=100", rootToken, nil, &listData); err != nil {
		panic(fmt.Sprintf("获取公寓列表失败：%v", err))
	}
	if listData.Total != 100 {
		panic(fmt.Sprintf("期望 100 个公寓，实际 %d 个（请先清空数据库再运行）", listData.Total))
	}

	for idx, b := range listData.Buildings {
		buildingID := b.ID
		// 2. 创建该公寓的楼栋管理员，用户名与密码均为公寓 id
		username := fmt.Sprintf("%d", buildingID)
		var userData struct {
			User struct {
				ID uint `json:"id"`
			} `json:"user"`
		}
		err := do("POST", "/api/admin/auth/create-building-admin", rootToken,
			map[string]interface{}{"username": username, "password": username, "building_id": buildingID}, &userData)
		if err != nil {
			ae, ok := err.(*apiError)
			if ok && ae.Status == http.StatusConflict {
				fmt.Printf("公寓 %d 的管理员 %s 已存在，跳过创建\n", buildingID, username)
			} else {
				panic(fmt.Sprintf("创建公寓 %d 的管理员失败：%v", buildingID, err))
			}
		}

		// 3. 以该公寓管理员身份登录
		token, err := login(username, username)
		if err != nil {
			panic(fmt.Sprintf("楼栋管理员 %s 登录失败：%v", username, err))
		}

		// 4. 生成 50 套房（25 单间 / 13 一室一厅 / 12 两房，随机打乱）
		layouts := make([]string, 0, roomsPerBuilding)
		for i := 0; i < studioCount; i++ {
			layouts = append(layouts, "单间")
		}
		for i := 0; i < oneBedCount; i++ {
			layouts = append(layouts, "一室一厅")
		}
		for i := 0; i < twoBedCount; i++ {
			layouts = append(layouts, "两房")
		}
		rng.Shuffle(len(layouts), func(i, j int) { layouts[i], layouts[j] = layouts[j], layouts[i] })

		roomIDs := make([]uint, 0, roomsPerBuilding)
		roomInfos := make([]map[string]interface{}, 0, roomsPerBuilding)
		unitPerFloor := make(map[int]int)
		for i := 0; i < roomsPerBuilding; i++ {
			layout := layouts[i]
			floor := 1 + rng.Intn(10)
			unitPerFloor[floor]++
			roomNo := fmt.Sprintf("%d%02d", floor, unitPerFloor[floor])

			rentPrice := rentForLayout(layout)
			depositMonths := uint(1 + rng.Intn(2))
			room := map[string]interface{}{
				"room_number":            roomNo,
				"floor":                  fmt.Sprintf("%d", floor),
				"layout":                 layout,
				"description":            "精装修，家电齐全，拎包入住。",
				"rent_price":             rentPrice,
				"deposit_months":         depositMonths,
				"management_fee":         round1(50 + rng.Float64()*150),
				"electricity_unit_price": round2(1.2 + rng.Float64()*0.6),
				"water_unit_price":       round2(4.5 + rng.Float64()*2.0),
			}

			var data struct {
				Room struct {
					ID uint `json:"id"`
				} `json:"room"`
			}
			if err := do("POST", "/api/building/rooms", token, room, &data); err != nil {
				panic(fmt.Sprintf("公寓 %d 创建房间 %s 失败：%v", buildingID, roomNo, err))
			}
			roomIDs = append(roomIDs, data.Room.ID)
			roomInfos = append(roomInfos, room)
		}

		// 5. 随机出租约 70% 的房间（走「设为已出租」接口，自动生成租客/合同/账单）
		rented := 0
		for i := 0; i < roomsPerBuilding; i++ {
			if rng.Float64() >= rentProbability {
				continue
			}
			info := roomInfos[i]
			rentPrice := info["rent_price"].(float64)
			depositMonths := info["deposit_months"].(uint)
			managementFee := info["management_fee"].(float64)

			start := randomDayInMonth(now.Year(), int(now.Month()))
			end := start.AddDate(0, 1+rng.Intn(3), rng.Intn(28))

			body := map[string]interface{}{
				"status":         "rented",
				"tenant_name":    randomName(),
				"tenant_phone":   randomPhone(),
				"rent_price":     rentPrice,
				"management_fee": managementFee,
				"deposit":        round1(rentPrice * float64(depositMonths)),
				"earnest_money":  0,
				"start_date":     start.Format("2006-01-02"),
				"end_date":       end.Format("2006-01-02"),
			}
			if err := do("PUT", fmt.Sprintf("/api/building/rooms/%d/status", roomIDs[i]), token, body, nil); err != nil {
				panic(fmt.Sprintf("公寓 %d 出租房间 %s 失败：%v", buildingID, info["room_number"], err))
			}
			rented++
		}

		fmt.Printf("公寓 %d/100（%s）：50 套房已创建，其中 %d 套已出租签约\n", idx+1, b.Name, rented)
	}
}

func randomDayInMonth(year, month int) time.Time {
	daysInMonth := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.Local).Day()
	return time.Date(year, time.Month(month), 1+rng.Intn(daysInMonth), 0, 0, 0, 0, time.Local)
}

func rentForLayout(layout string) float64 {
	var min, max float64
	switch layout {
	case "单间":
		min, max = 1000, 2500
	case "一室一厅":
		min, max = 2600, 3800
	case "两房":
		min, max = 3800, 5000
	}
	steps := int((max-min)/100) + 1
	return min + float64(rng.Intn(steps)*100)
}

func randomName() string {
	return surnames[rng.Intn(len(surnames))] + givenNames[rng.Intn(len(givenNames))] + givenNames[rng.Intn(len(givenNames))]
}

func randomPhone() string {
	var phone string
	for {
		phone = "13" + string(rune('0'+rng.Intn(8))) + fmt.Sprintf("%08d", rng.Intn(100000000))
		if !usedPhone[phone] {
			usedPhone[phone] = true
			return phone
		}
	}
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}