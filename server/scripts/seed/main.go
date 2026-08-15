// seed 通过 HTTP API 生成演示数据：100 个公寓（含房东信息）+ 每栋 50 套房 + 约 70% 房间签约出租。
// 所有操作（创建公寓、创建楼栋管理员、登录、创建房间、出租签约）全部走真实 HTTP 接口。
//
// 默认约定：
//   - 每个公寓创建 1 个管理账号，用户名与密码均为公寓 id（如公寓 3 → 3 / 3）
//   - 公寓一律全套餐（full），签约时间（contract_date）为跑脚本当天，到期时间自动为一年后
//
// 前置条件：
//  1. 后端服务已在默认地址运行（可用环境变量 SEED_BASE_URL 覆盖，默认 http://127.0.0.1:8081）
//  2. 数据库中已有超级管理员 root（首次启动自动创建）
//
// 运行方式（在 server 目录下）：go run ./scripts/seed
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

const defaultBaseURL = "http://127.0.0.1:8081"

var baseURL = os.Getenv("SEED_BASE_URL")

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

// do 发送请求并解析统一响应，失败时自动重试（网络错误与 5xx）
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
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		if err := doOnce(method, path, token, bodyBytes, out); err != nil {
			lastErr = err
			ae, ok := err.(*apiError)
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