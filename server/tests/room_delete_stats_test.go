package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
	"rental-server/utils"
)

// TestRoomDelete_CleansStatsAndBillRoom 验证删房间时：
//   - 该房间的 room_detail 明细被删除；
//   - 楼栋 stat_daily 的 pv/detail_pv 按房间事件数扣减，平台行不变；
//   - 账单保留但 room_id 显式置空（软删不触发外键 SET NULL）。
func TestRoomDelete_CleansStatsAndBillRoom(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	now := time.Now()
	todayKey := now.Format("2006-01-02")

	room := models.Room{BuildingID: f.A.ID, RoomNumber: "DEL1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)

	// 房间 2 条 room_detail + 楼栋 1 条 building_detail
	require.NoError(t, utils.ApplyPageViewBatch(app.DB, []models.PageView{
		{PageType: "room_detail", ResourceID: room.ID, BuildingID: f.A.ID, IP: "10.0.0.1", CreatedAt: now},
		{PageType: "room_detail", ResourceID: room.ID, BuildingID: f.A.ID, IP: "10.0.0.2", CreatedAt: now},
		{PageType: "building_detail", ResourceID: f.A.ID, BuildingID: f.A.ID, IP: "10.0.0.3", CreatedAt: now},
	}))

	var before models.StatDaily
	require.NoError(t, app.DB.Where("day = ? AND building_id = ?", todayKey, f.A.ID).First(&before).Error)
	require.EqualValues(t, 3, before.PV)
	require.EqualValues(t, 3, before.DetailPV)

	bill := models.Bill{
		BillNo: "ROOMDEL-1", Type: "income", Amount: 100, PaidStatus: "unpaid",
		BuildingID: f.A.ID, RoomID: &room.ID, BillDate: todayKey,
	}
	require.NoError(t, app.DB.Create(&bill).Error)

	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil)
	del.RequireOK(t)

	require.Zero(t, countWhere(t, app.DB, &models.PageView{}, "page_type = ? AND resource_id = ?", "room_detail", room.ID))

	var after models.StatDaily
	require.NoError(t, app.DB.Where("day = ? AND building_id = ?", todayKey, f.A.ID).First(&after).Error)
	require.EqualValues(t, 1, after.PV, "楼栋行应只扣减房间的 2 条 room_detail")
	require.EqualValues(t, 1, after.DetailPV)

	var platform models.StatDaily
	require.NoError(t, app.DB.Where("day = ? AND building_id = 0", todayKey).First(&platform).Error)
	require.EqualValues(t, 3, platform.PV, "平台行不随房间删除回落")

	var got models.Bill
	require.NoError(t, app.DB.First(&got, bill.ID).Error)
	require.Nil(t, got.RoomID, "账单保留但 room_id 应置空")
}
