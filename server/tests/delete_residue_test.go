package tests

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"rental-server/models"
	"rental-server/scheduler"
	"rental-server/services"
	"rental-server/tests/testkit"
)

// pushDeletedAtBack 把软删除时间往前推，模拟「已过 90 天清理窗口」。
func pushDeletedAtBack(t *testing.T, db *gorm.DB, days int) {
	t.Helper()
	old := time.Now().AddDate(0, 0, -days)
	for _, m := range []interface{}{
		&models.Building{}, &models.Room{}, &models.Tenant{}, &models.RentalContract{},
		&models.Bill{}, &models.Task{}, &models.Shareholder{}, &models.Dividend{},
		&models.BuildingRenewal{}, &models.BuildingDepositLog{}, &models.User{},
	} {
		require.NoError(t, db.Unscoped().Model(m).
			Where("deleted_at IS NOT NULL").UpdateColumn("deleted_at", old).Error)
	}
}

// TestDelete_SharedMediaNotRemoved 删房间不得误删其他房间仍在引用的共享媒体。
// 回归：旧实现用 `WHERE file_path = ? AND deleted_at IS NULL` 统计引用，
// room_media 没有 deleted_at 列 → SQL 报错、错误被忽略、count 恒为 0 → 共享文件被误删。
func TestDelete_SharedMediaNotRemoved(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	roomA := models.Room{BuildingID: f.A.ID, RoomNumber: "SH1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&roomA).Error)
	roomB := models.Room{BuildingID: f.A.ID, RoomNumber: "SH2", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&roomB).Error)

	key := fmt.Sprintf("buildings/%d/rooms/shared.png", f.A.ID)
	thumbKey := fmt.Sprintf("buildings/%d/rooms/thumb_shared.png", f.A.ID)
	p := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(key))
	tp := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(thumbKey))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("img"), 0o644))
	require.NoError(t, os.WriteFile(tp, []byte("thumb"), 0o644))

	for _, rid := range []uint{roomA.ID, roomB.ID} {
		require.NoError(t, app.DB.Create(&models.RoomMedia{
			RoomID: rid, Type: "image", Category: "gallery",
			FilePath: key, ThumbnailPath: thumbKey, Status: "ready",
		}).Error)
	}

	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", roomA.ID), f.TokenA, nil)
	del.RequireOK(t)

	_, err := os.Stat(p)
	require.NoError(t, err, "其他房间仍在引用的共享原图不应被删除")
	_, err = os.Stat(tp)
	require.NoError(t, err, "其他房间仍在引用的共享缩略图不应被删除")

	var n int64
	require.NoError(t, app.DB.Model(&models.RoomMedia{}).Where("file_path = ?", key).Count(&n).Error)
	require.EqualValues(t, 1, n, "房间 B 的媒体记录应保留")
}

// TestDelete_RoomDeletesOwnMediaOnly 删房间仍要清理「本房间独占」的文件。
func TestDelete_RoomDeletesOwnMediaOnly(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	room := models.Room{BuildingID: f.A.ID, RoomNumber: "ONLY1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)
	key := fmt.Sprintf("buildings/%d/rooms/only.png", f.A.ID)
	p := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(key))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("img"), 0o644))
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: room.ID, Type: "image", Category: "gallery", FilePath: key, Status: "ready",
	}).Error)

	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil).RequireOK(t)

	_, err := os.Stat(p)
	require.True(t, os.IsNotExist(err), "独占媒体文件应随房间删除")
}

// TestCleanup_SoftDeletedResidue 删公寓后，90 天清理必须把关联数据全部清干净。
// 回归：历史实现把表按「公寓在前」排列且吞掉全部错误，
// 外键（ON DELETE NO ACTION）挡住父行删除 → 每轮空转、数据永久残留。
func TestCleanup_SoftDeletedResidue(t *testing.T) {
	app := testkit.NewApp(t)

	b := models.Building{Name: "残留楼", Status: "active"}
	require.NoError(t, app.DB.Create(&b).Error)
	room := models.Room{BuildingID: b.ID, RoomNumber: "A1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)
	tenant := models.Tenant{Name: "张三", Phone: "13900000000"}
	require.NoError(t, app.DB.Create(&tenant).Error)
	contract := models.RentalContract{RoomID: room.ID, BuildingID: b.ID, TenantID: tenant.ID,
		StartDate: "2026-01-01", EndDate: "2026-12-31", Status: "ended"}
	require.NoError(t, app.DB.Create(&contract).Error)
	bill := models.Bill{BillNo: "B1", Type: "income", Amount: 100, BuildingID: b.ID, RoomID: &room.ID, BillDate: "2026-01-01"}
	require.NoError(t, app.DB.Create(&bill).Error)
	require.NoError(t, app.DB.Create(&models.BillEvidence{
		BillID: bill.ID, BuildingID: b.ID, FileKey: "buildings/1_x/bills/a.png",
	}).Error)
	task := models.Task{BuildingID: &b.ID, RoomID: &room.ID, Title: "维修", Type: "repair", Status: "pending", Scope: "building"}
	require.NoError(t, app.DB.Create(&task).Error)
	sh := models.Shareholder{BuildingID: b.ID, Name: "李四", ShareRatio: 100}
	require.NoError(t, app.DB.Create(&sh).Error)

	require.NoError(t, services.NewBuildingService(app.DB).Delete(b.ID))

	// 账单保留但房间引用必须断开
	var roomID *uint
	require.NoError(t, app.DB.Unscoped().Model(&models.Bill{}).Where("bill_no = ?", "B1").
		Select("room_id").Scan(&roomID).Error)
	require.Nil(t, roomID, "删公寓后账单不应再指向已删房间")

	// 模拟 90 天后：软删时间往前推
	pushDeletedAtBack(t, app.DB, 100)
	require.NoError(t, models.CleanupSoftDeleted(app.DB, 90))

	countAll := func(m interface{}) int64 {
		var n int64
		require.NoError(t, app.DB.Unscoped().Model(m).Count(&n).Error)
		return n
	}
	require.EqualValues(t, 0, countAll(&models.Building{}), "公寓行应被清理")
	require.EqualValues(t, 0, countAll(&models.Room{}), "房间行应被清理")
	require.EqualValues(t, 0, countAll(&models.Bill{}), "账单应被清理")
	require.EqualValues(t, 0, countAll(&models.RentalContract{}), "合同应被清理")
	require.EqualValues(t, 0, countAll(&models.Tenant{}), "租客应被清理")
	require.EqualValues(t, 0, countAll(&models.Task{}), "待办应被清理")
	require.EqualValues(t, 0, countAll(&models.Shareholder{}), "股东应被清理")
	require.EqualValues(t, 0, countAll(&models.BillEvidence{}), "账单凭证行应随账单一起清理")
}

// TestCleanup_OrphanRowsRemoved 清理任务要顺手清掉父行已不存在的孤儿行。
// 场景来自存量库（外键是 NO ACTION，历史库甚至没有外键），需要临时关闭外键约束
// 才能造出「子行指向不存在的父行」这种脏数据。
func TestCleanup_OrphanRowsRemoved(t *testing.T) {
	app := testkit.NewApp(t)

	sqlDB, err := app.DB.DB()
	require.NoError(t, err)
	// 单连接 + 关闭外键：造出历史库才可能出现的孤儿行
	sqlDB.SetMaxOpenConns(1)
	_, err = sqlDB.Exec("PRAGMA foreign_keys = OFF")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = sqlDB.Exec("PRAGMA foreign_keys = ON")
		sqlDB.SetMaxOpenConns(4)
	})

	b := models.Building{Name: "孤儿楼", Status: "active"}
	require.NoError(t, app.DB.Create(&b).Error)
	room := models.Room{BuildingID: b.ID, RoomNumber: "A1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)
	// 凭证行指向不存在的账单（历史库无外键时的残留）
	require.NoError(t, app.DB.Create(&models.BillEvidence{
		BillID: 999999, BuildingID: b.ID, FileKey: "buildings/1_x/bills/orphan.png",
	}).Error)
	// 媒体行指向不存在的房间
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: 999999, Type: "image", Category: "gallery", FilePath: "x.png", Status: "ready",
	}).Error)

	require.NoError(t, models.CleanupSoftDeleted(app.DB, 90))

	var n int64
	require.NoError(t, app.DB.Model(&models.BillEvidence{}).Count(&n).Error)
	require.EqualValues(t, 0, n, "孤儿凭证行应被清理")
	require.NoError(t, app.DB.Model(&models.RoomMedia{}).Count(&n).Error)
	require.EqualValues(t, 0, n, "孤儿媒体行应被清理")
}

// TestDelete_ReservedContractBlocksDelete 预订中（已收定金）的房间/公寓不可删。
func TestDelete_ReservedContractBlocksDelete(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// A103 有一份 reserved 合同（见 fixture）
	require.NotZero(t, f.Rooms["A103"].ID)
	r := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["A103"].ID), f.TokenA, nil)
	r.RequireError(t, http.StatusBadRequest, 400)

	// 公寓维度同理（业务码 1003 = 存在未结束合同）
	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, nil)
	del.RequireError(t, http.StatusConflict, 1003)
}

// TestDelete_BuildingCoverReplaced 封面被替换后旧文件应被回收；
// 被其他公寓引用的封面必须保留。
func TestDelete_BuildingCoverReplaced(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	oldKey := fmt.Sprintf("buildings/%d/cover/old.png", f.A.ID)
	newKey := fmt.Sprintf("buildings/%d/cover/new.png", f.A.ID)
	p := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(oldKey))
	np := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(newKey))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o644))
	require.NoError(t, os.WriteFile(np, []byte("new"), 0o644))
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.A.ID).
		Update("cover_image", oldKey).Error)
	// B 栋也引用同一张封面：此时 A 换封面不应删掉它
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.B.ID).
		Update("cover_image", oldKey).Error)

	upd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, map[string]any{
		"cover_image": newKey,
	})
	upd.RequireOK(t)
	_, err := os.Stat(p)
	require.NoError(t, err, "被其他公寓引用的封面不应被删除")

	// B 栋不再引用后，A 再次换封面 → A 的上一张封面（newKey）应被回收
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.B.ID).
		Update("cover_image", "").Error)
	upd2 := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, map[string]any{
		"cover_image": "buildings/x/cover/other.png",
	})
	upd2.RequireOK(t)
	_, err = os.Stat(np)
	require.True(t, os.IsNotExist(err), "无人引用的旧封面应被回收")
}

// TestDelete_ClearBuildingCover 清空封面（cover_image 传空串）应生效并回收旧文件。
func TestDelete_ClearBuildingCover(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	key := fmt.Sprintf("buildings/%d/cover/clear.png", f.A.ID)
	p := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(key))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.A.ID).
		Update("cover_image", key).Error)

	r := app.Client.Do(http.MethodPut, "/api/building/info", f.TokenA, map[string]any{"cover_image": ""})
	r.RequireOK(t)

	var cover string
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.A.ID).
		Select("cover_image").Scan(&cover).Error)
	require.Empty(t, cover, "封面应被清空")
	_, err := os.Stat(p)
	require.True(t, os.IsNotExist(err), "清空封面后旧文件应被回收")
}

// TestSchema_ForeignKeyActions 新建库的外键必须带 ON DELETE 子句。
// 回归：模型曾把 constraint:OnDelete:CASCADE 写在标量外键字段上，
// GORM v2 只读关联字段上的同名 tag，实际建出来的是无 ON DELETE 的外键，
// 于是「父行有子行时硬删父行」必然报 FOREIGN KEY constraint failed。
func TestSchema_ForeignKeyActions(t *testing.T) {
	app := testkit.NewApp(t)

	type fkRow struct {
		Table    string
		FromCol  string
		RefTable string
		OnDelete string
	}
	var rows []fkRow
	require.NoError(t, app.DB.Raw(`SELECT m.name AS "table", p."from" AS from_col, p."table" AS ref_table,
		p.on_delete AS on_delete FROM sqlite_master m
		JOIN pragma_foreign_key_list(m.name) p
		WHERE m.type='table' AND m.name NOT LIKE 'sqlite_%'`).Scan(&rows).Error)

	onDelete := map[string]string{}
	for _, r := range rows {
		onDelete[r.Table+"."+r.FromCol] = r.OnDelete
	}
	require.Equal(t, "CASCADE", onDelete["room_media.room_id"], "房间媒体应随房间级联删除")
	require.Equal(t, "CASCADE", onDelete["bill_evidences.bill_id"], "账单凭证应随账单级联删除")
	require.Equal(t, "CASCADE", onDelete["rental_contracts.room_id"], "合同应随房间级联删除")
	require.Equal(t, "SET NULL", onDelete["bills.room_id"], "账单的房间引用应置空而非级联删除")
	require.Equal(t, "SET NULL", onDelete["tasks.room_id"], "待办的房间引用应置空")
}

// TestMediaGC_OrphanFiles 媒体孤儿文件 GC：无人引用且超过宽限期的文件要删，
// 仍被引用的与宽限期内的要保留。
func TestMediaGC_OrphanFiles(t *testing.T) {
	app := testkit.NewApp(t)

	b := models.Building{Name: "gc-b", Status: "active"}
	require.NoError(t, app.DB.Create(&b).Error)
	room := models.Room{BuildingID: b.ID, RoomNumber: "A1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)

	refKey := fmt.Sprintf("buildings/%d/rooms/ref.png", b.ID)
	orphanKey := fmt.Sprintf("buildings/%d/rooms/orphan.png", b.ID)
	freshKey := fmt.Sprintf("buildings/%d/rooms/fresh.png", b.ID)
	write := func(key, content string) string {
		p := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(key))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}
	refPath := write(refKey, "ref")
	orphanPath := write(orphanKey, "orphan")
	freshPath := write(freshKey, "fresh")
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: room.ID, Type: "image", Category: "gallery", FilePath: refKey, Status: "ready",
	}).Error)

	// 孤儿文件与宽限期外：mtime 设为 30 天前；fresh 保持现在
	old := time.Now().AddDate(0, 0, -30)
	require.NoError(t, os.Chtimes(orphanPath, old, old))

	summary := scheduler.CleanupMediaFiles(app.DB, app.Cfg)
	require.Contains(t, summary, "媒体GC")

	_, err := os.Stat(refPath)
	require.NoError(t, err, "仍被引用的媒体文件必须保留")
	_, err = os.Stat(orphanPath)
	require.True(t, os.IsNotExist(err), "超过宽限期的孤儿文件应被清理")
	_, err = os.Stat(freshPath)
	require.NoError(t, err, "宽限期内的文件不应被清理")
}
