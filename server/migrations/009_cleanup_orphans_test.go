package migrations

import (
	"testing"

	"rental-server/models"
)

// TestCleanupOrphans 验证 009 能清掉存量库里的孤儿行：
// 凭证行指向已不存在的账单、媒体行指向已不存在的房间、待办指向已不存在的房间/公寓。
func TestCleanupOrphans(t *testing.T) {
	db := newTestDB(t)
	if err := db.AutoMigrate(&models.Building{}, &models.Room{}, &models.Bill{},
		&models.BillEvidence{}, &models.RoomMedia{}, &models.Task{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 造孤儿行需要关闭外键约束（新库的外键带 ON DELETE，插不进孤儿行）
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取连接池失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("关闭外键失败: %v", err)
	}
	defer func() {
		_, _ = sqlDB.Exec("PRAGMA foreign_keys = ON")
		sqlDB.SetMaxOpenConns(4)
	}()

	b := models.Building{Name: "迁移楼"}
	if err := db.Create(&b).Error; err != nil {
		t.Fatalf("建公寓失败: %v", err)
	}
	room := models.Room{BuildingID: b.ID, RoomNumber: "A1", Floor: "1", Layout: "单间"}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("建房间失败: %v", err)
	}
	// 孤儿凭证行：账单不存在
	if err := db.Create(&models.BillEvidence{
		BillID: 999999, BuildingID: b.ID, FileKey: "buildings/1_x/bills/orphan.png",
	}).Error; err != nil {
		t.Fatalf("建孤儿凭证失败: %v", err)
	}
	// 孤儿媒体行：房间不存在
	if err := db.Create(&models.RoomMedia{
		RoomID: 999999, Type: "image", Category: "gallery", FilePath: "x.png", Status: "ready",
	}).Error; err != nil {
		t.Fatalf("建孤儿媒体失败: %v", err)
	}
	// 待办指向不存在的房间/公寓
	if err := db.Create(&models.Task{
		Title: "孤儿待办", Type: "repair", Status: "pending", Scope: "building",
		RoomID: &[]uint{999999}[0], BuildingID: &[]uint{999999}[0],
	}).Error; err != nil {
		t.Fatalf("建孤儿待办失败: %v", err)
	}

	if err := upCleanupOrphans(db); err != nil {
		t.Fatalf("清理孤儿失败: %v", err)
	}

	count := func(m interface{}) int64 {
		var n int64
		if err := db.Model(m).Count(&n).Error; err != nil {
			t.Fatalf("统计失败: %v", err)
		}
		return n
	}
	if count(&models.BillEvidence{}) != 0 {
		t.Fatal("孤儿凭证行未被清理")
	}
	if count(&models.RoomMedia{}) != 0 {
		t.Fatal("孤儿媒体行未被清理")
	}
	// 待办本身保留（历史与审计价值），但指向已不存在父行的引用要置空
	var task models.Task
	if err := db.Where("title = ?", "孤儿待办").First(&task).Error; err != nil {
		t.Fatalf("待办丢失: %v", err)
	}
	if task.RoomID != nil || task.BuildingID != nil {
		t.Fatal("待办的房间/公寓引用未置空")
	}

	// 幂等：再执行一次不应报错
	if err := upCleanupOrphans(db); err != nil {
		t.Fatalf("重复执行失败: %v", err)
	}
}
