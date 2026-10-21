// handlers 包的集成测试脚手架：内存数据库 + 带登录身份的 gin 引擎。
//
// 覆盖的是「钱相关」的核心状态机（出租/退租/预订/账单/分红/退租待办），
// 断言的是数据库最终状态（合同、账单、房间状态、待办），而不是接口文案。
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"rental-server/logger"
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logger.Log = zerolog.Nop()
	os.Exit(m.Run())
}

// testEnv 测试环境。
type testEnv struct {
	db  *gorm.DB
	eng *gin.Engine
}

// newTestEnv 建立独立的临时数据库与带身份的测试引擎。
// 身份通过请求头 X-Building 注入 building_id，模拟房东后台的登录态。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := models.AutoMigrate(db); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 关闭连接，否则 Windows 下临时目录清理会因文件被占用而失败
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})

	eng := gin.New()
	eng.Use(func(c *gin.Context) {
		c.Set("user_id", uint(1))
		c.Set("username", "tester")
		c.Set("role", "building_admin")
		bid, _ := strconv.ParseUint(c.GetHeader("X-Building"), 10, 64)
		c.Set("building_id", uint(bid))
		c.Next()
	})
	return &testEnv{db: db, eng: eng}
}

// apiResp 接口统一响应结构。
type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// do 发起一次请求并返回响应记录器。
func (e *testEnv) do(t *testing.T, method, path string, buildingID uint, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("序列化请求体失败: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Building", strconv.FormatUint(uint64(buildingID), 10))
	w := httptest.NewRecorder()
	e.eng.ServeHTTP(w, req)
	return w
}

// parse 解析响应。
func parse(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	var resp apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, w.Body.String())
	}
	return resp
}

// expectOK 断言业务成功（code=0）。
func expectOK(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	resp := parse(t, w)
	if resp.Code != 0 {
		t.Fatalf("期望成功，实际 code=%d message=%s body=%s", resp.Code, resp.Message, w.Body.String())
	}
	return resp
}

// expectFail 断言业务失败，并检查错误码（非 0）。
func expectFail(t *testing.T, w *httptest.ResponseRecorder) apiResp {
	t.Helper()
	resp := parse(t, w)
	if resp.Code == 0 {
		t.Fatalf("期望失败，实际成功: body=%s", w.Body.String())
	}
	return resp
}

// seedBuilding 造公寓。
func seedBuilding(t *testing.T, db *gorm.DB, id uint) models.Building {
	t.Helper()
	b := models.Building{ID: id, Name: "测试公寓" + strconv.Itoa(int(id)), Status: "active"}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("造公寓数据失败: %v", err)
	}
	return b
}

// seedRoom 造房间。
func seedRoom(t *testing.T, db *gorm.DB, id, buildingID uint, status string) models.Room {
	t.Helper()
	r := models.Room{ID: id, BuildingID: buildingID, RoomNumber: strconv.Itoa(int(id)), Floor: "1", Layout: "一室一厅", Status: status}
	if err := db.Create(&r).Error; err != nil {
		t.Fatalf("造房间数据失败: %v", err)
	}
	return r
}

// seedTenant 造租客。
func seedTenant(t *testing.T, db *gorm.DB, name string) models.Tenant {
	t.Helper()
	tenant := models.Tenant{Name: name, Phone: "13800000000"}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatalf("造租客数据失败: %v", err)
	}
	return tenant
}

// seedActiveContract 造生效中的合同。
func seedActiveContract(t *testing.T, db *gorm.DB, id, roomID, buildingID, tenantID uint, deposit float64) models.RentalContract {
	t.Helper()
	ct := models.RentalContract{
		ID: id, RoomID: roomID, BuildingID: buildingID, TenantID: tenantID,
		RentPrice: 2000, ManagementFee: 200, Deposit: deposit,
		StartDate: "2026-01-01", EndDate: "2026-12-31", Status: "active",
	}
	if err := db.Create(&ct).Error; err != nil {
		t.Fatalf("造合同数据失败: %v", err)
	}
	return ct
}

// seedBill 造账单。
func seedBill(t *testing.T, db *gorm.DB, id, buildingID uint, roomID *uint, billType, subtype string, amount float64, billDate string) models.Bill {
	t.Helper()
	b := models.Bill{
		ID: id, BillNo: "B" + strconv.Itoa(int(id)), BuildingID: buildingID, RoomID: roomID,
		Type: billType, Subtype: subtype, Amount: amount, BillDate: billDate, PaidStatus: "paid",
	}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("造账单数据失败: %v", err)
	}
	return b
}

// countBills 统计满足条件的账单数量。
func countBills(t *testing.T, db *gorm.DB, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&models.Bill{}).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatalf("统计账单失败: %v", err)
	}
	return n
}

// getRoom 取房间当前状态。
func getRoom(t *testing.T, db *gorm.DB, id uint) models.Room {
	t.Helper()
	var r models.Room
	if err := db.First(&r, id).Error; err != nil {
		t.Fatalf("读取房间失败: %v", err)
	}
	return r
}

// itoa 主键转字符串，拼接请求路径用。
func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
