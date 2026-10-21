package audit

import (
	"net/http/httptest"
	"testing"

	"rental-server/models"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	return db
}

// newCtx 构造带登录信息的测试请求上下文。
func newCtx() *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/api/admin/test", nil)
	c.Set("user_id", uint(7))
	c.Set("username", "root")
	c.Set("role", "super_admin")
	c.Set("building_id", uint(3))
	return c
}

func TestLogPersistsAuditRecord(t *testing.T) {
	db := newTestDB(t)
	Init(db)
	defer Init(nil)

	Log(newCtx(), ActionUpdate, ResourceRoom, "12", "修改房间 字段=rent_price")

	var rec models.AuditLog
	if err := db.First(&rec).Error; err != nil {
		t.Fatalf("未落库审计记录: %v", err)
	}
	if rec.Action != ActionUpdate || rec.Resource != ResourceRoom || rec.ResourceID != "12" {
		t.Fatalf("审计字段不符: %+v", rec)
	}
	if rec.UserID != 7 || rec.Username != "root" {
		t.Fatalf("操作人不符: %+v", rec)
	}
	if rec.BuildingID == nil || *rec.BuildingID != 3 {
		t.Fatalf("公寓 id 不符: %+v", rec.BuildingID)
	}
	if rec.Detail != "修改房间 字段=rent_price" {
		t.Fatalf("明细不符: %q", rec.Detail)
	}
}

func TestLogSystemHasNoActor(t *testing.T) {
	db := newTestDB(t)
	Init(db)
	defer Init(nil)

	LogSystem("scheduler", ResourceJob, "rent_bills", "创建 0 条")

	var rec models.AuditLog
	if err := db.First(&rec).Error; err != nil {
		t.Fatalf("未落库审计记录: %v", err)
	}
	if rec.UserID != 0 || rec.Username != "system" {
		t.Fatalf("系统操作人不应是真实用户: %+v", rec)
	}
	if rec.Action != "scheduler" || rec.Resource != ResourceJob {
		t.Fatalf("审计字段不符: %+v", rec)
	}
}

func TestLogWithoutDBStillWorks(t *testing.T) {
	Init(nil)
	// 未注入数据库时不应 panic（仅日志文件输出）
	Log(newCtx(), ActionCreate, ResourceBuilding, "1", "创建公寓")
}

func TestKeysSorted(t *testing.T) {
	got := Keys(map[string]interface{}{"b": 1, "a": 2, "c": 3})
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("键名未排序: %v", got)
	}
}
