package tests

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

func writeMediaFile(t *testing.T, app *testkit.App, key string) string {
	t.Helper()
	p := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(key))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("media"), 0o644))
	return p
}

// TestBuildingDelete_MediaFiles 验证删公寓时：
//   - 只被本公寓引用的房间媒体/封面物理文件被删除；
//   - 被其他楼栋复用的共享媒体文件保留；
//   - 已软删房间的 room_media 行也被清理（否则文件永远删不掉）。
func TestBuildingDelete_MediaFiles(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	b := models.Building{Name: "删除媒体楼", Status: "active"}
	require.NoError(t, app.DB.Create(&b).Error)
	room := models.Room{BuildingID: b.ID, RoomNumber: "D1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)

	ownKey := fmt.Sprintf("buildings/%d/tmp/own.png", b.ID)
	ownPath := writeMediaFile(t, app, ownKey)
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: room.ID, Type: "image", Category: "gallery", FilePath: ownKey, Status: "ready",
	}).Error)

	sharedKey := fmt.Sprintf("buildings/%d/tmp/shared.png", b.ID)
	sharedPath := writeMediaFile(t, app, sharedKey)
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: room.ID, Type: "image", Category: "gallery", FilePath: sharedKey, Status: "ready",
	}).Error)

	coverKey := fmt.Sprintf("buildings/%d/tmp/cover.png", b.ID)
	coverPath := writeMediaFile(t, app, coverKey)
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", b.ID).Update("cover_image", coverKey).Error)

	// 已软删房间的媒体行：删除公寓时必须一并清掉
	softRoom := models.Room{BuildingID: b.ID, RoomNumber: "D2", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&softRoom).Error)
	softKey := fmt.Sprintf("buildings/%d/tmp/soft.png", b.ID)
	softPath := writeMediaFile(t, app, softKey)
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: softRoom.ID, Type: "image", Category: "gallery", FilePath: softKey, Status: "ready",
	}).Error)
	require.NoError(t, app.DB.Delete(&softRoom).Error) // 软删房间，room_media 行保留

	// 另一个楼栋复用 sharedKey：删除 b 后该文件必须保留
	other := models.Building{Name: "复用媒体楼", Status: "active"}
	require.NoError(t, app.DB.Create(&other).Error)
	otherRoom := models.Room{BuildingID: other.ID, RoomNumber: "O1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&otherRoom).Error)
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: otherRoom.ID, Type: "image", Category: "gallery", FilePath: sharedKey, Status: "ready",
	}).Error)

	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/buildings/%d", b.ID), f.TokenRoot, nil)
	del.RequireOK(t)

	// 只被本公寓引用的文件应删除
	_, err := os.Stat(ownPath)
	require.True(t, os.IsNotExist(err), "本公寓独享的房间媒体文件应被删除")
	_, err = os.Stat(coverPath)
	require.True(t, os.IsNotExist(err), "封面文件应被删除")
	_, err = os.Stat(softPath)
	require.True(t, os.IsNotExist(err), "已软删房间的媒体文件也应被删除")

	// 被其他楼栋复用的共享文件必须保留
	_, err = os.Stat(sharedPath)
	require.NoError(t, err, "共享媒体文件仍被其他楼栋引用，不能删除")

	// 已软删房间的 room_media 行应被清理
	require.Zero(t, countWhere(t, app.DB, &models.RoomMedia{}, "room_id = ?", softRoom.ID))
}
