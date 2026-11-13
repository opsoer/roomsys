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

// TestBuildingDeleteCascadeStats 验证删除公寓的监测数据级联语义（真实写入链路）：
// utils.ApplyPageViewBatch 落库 → DELETE /api/admin/buildings/:id →
// 该公寓明细与楼栋日行清零（排行/详情立即消失），平台级累计不回落，其他公寓数据不受影响。
func TestBuildingDeleteCascadeStats(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	now := time.Now()
	todayKey := now.Format("2006-01-02")

	// 造一个与 fixture 无关的公寓 + 两个房间（不经过 API，聚焦统计本身）
	building := models.Building{Name: "统计测试公寓", Status: "active"}
	require.NoError(t, app.DB.Create(&building).Error)
	roomKeep := models.Room{BuildingID: building.ID, RoomNumber: "S1", Floor: "1", Layout: "单间", Status: "vacant"}
	roomGone := models.Room{BuildingID: building.ID, RoomNumber: "S2", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&roomKeep).Error)
	require.NoError(t, app.DB.Create(&roomGone).Error)

	// 其他公寓（f.A）也留一份监测数据，删除后必须原样保留
	require.NoError(t, utils.ApplyPageViewBatch(app.DB, []models.PageView{
		{PageType: "building_detail", BuildingID: f.A.ID, IP: "7.7.7.7", CreatedAt: now},
	}))

	// 目标公寓的监测数据：走真实写入链路（明细 + 四张聚合表同步落库）
	require.NoError(t, utils.ApplyPageViewBatch(app.DB, []models.PageView{
		{PageType: "building_detail", ResourceID: building.ID, BuildingID: building.ID, IP: "9.9.9.1", CreatedAt: now},
		{PageType: "room_detail", ResourceID: roomKeep.ID, BuildingID: building.ID, IP: "9.9.9.2", CreatedAt: now},
		{PageType: "room_detail", ResourceID: roomGone.ID, BuildingID: building.ID, IP: "9.9.9.3", CreatedAt: now},
		{PageType: "landlord_view", ResourceID: building.ID, BuildingID: building.ID, IP: "9.9.9.4", CreatedAt: now},
	}))

	// 软删除 roomGone：房间排行不得漏出软删除房间（明细行还在）
	require.NoError(t, app.DB.Delete(&roomGone).Error)
	detail := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/admin/stats/building/%d", building.ID), f.TokenRoot, nil)
	detail.RequireOK(t)
	var detailData struct {
		BuildingStats struct {
			PV       int64 `json:"pv"`
			RoomRank []struct {
				RoomID uint `json:"room_id"`
			} `json:"room_rank"`
		} `json:"building_stats"`
	}
	detail.Decode(t, &detailData)
	require.EqualValues(t, 3, detailData.BuildingStats.PV, "楼栋详情 PV 应来自 stat_daily（detail 两类口径）")
	require.Len(t, detailData.BuildingStats.RoomRank, 1, "房间排行应只含未删除的房间")
	require.Equal(t, roomKeep.ID, detailData.BuildingStats.RoomRank[0].RoomID)

	// 删除前：总 PV=5（目标公寓 4 + f.A 1），排行含两个楼栋
	overviewBefore := app.Client.Do(http.MethodGet, "/api/admin/stats/overview", f.TokenRoot, nil)
	overviewBefore.RequireOK(t)
	var before struct {
		Overview struct {
			TotalPV      int64 `json:"total_pv"`
			TotalUV      int64 `json:"total_uv"`
			BuildingRank []struct {
				BuildingID uint `json:"building_id"`
			} `json:"building_rank"`
		} `json:"overview"`
	}
	overviewBefore.Decode(t, &before)
	require.EqualValues(t, 5, before.Overview.TotalPV)
	require.EqualValues(t, 5, before.Overview.TotalUV)
	require.Len(t, before.Overview.BuildingRank, 2)
	require.Equal(t, building.ID, before.Overview.BuildingRank[0].BuildingID, "detail_pv 高者应排第一")

	// 删除公寓
	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/buildings/%d", building.ID), f.TokenRoot, nil)
	del.RequireOK(t)

	// 明细与楼栋日行清零
	require.Zero(t, countWhere(t, app.DB, &models.PageView{}, "building_id = ?", building.ID))
	require.Zero(t, countWhere(t, app.DB, &models.StatDaily{}, "building_id = ?", building.ID))

	// 其他公寓的明细与楼栋日行原样保留
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.PageView{}, "building_id = ?", f.A.ID))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.StatDaily{}, "building_id = ?", f.A.ID))

	// 平台行与平台累计不变：总 PV/UV、平台日行都只增不减
	var platformRow models.StatDaily
	require.NoError(t, app.DB.Where("day = ? AND building_id = 0", todayKey).First(&platformRow).Error)
	require.EqualValues(t, 5, platformRow.PV)
	require.EqualValues(t, 5, countWhere(t, app.DB, &models.StatIP{}, "1 = 1"))
	var totals models.StatTotal
	require.NoError(t, app.DB.First(&totals, 1).Error)
	require.EqualValues(t, 5, totals.TotalPV)
	require.EqualValues(t, 1, totals.TotalLandlordView)

	overviewAfter := app.Client.Do(http.MethodGet, "/api/admin/stats/overview", f.TokenRoot, nil)
	overviewAfter.RequireOK(t)
	var after struct {
		Overview struct {
			TotalPV      int64 `json:"total_pv"`
			TotalUV      int64 `json:"total_uv"`
			BuildingRank []struct {
				BuildingID uint `json:"building_id"`
			} `json:"building_rank"`
		} `json:"overview"`
	}
	overviewAfter.Decode(t, &after)
	require.EqualValues(t, 5, after.Overview.TotalPV, "删除公寓后总 PV 不应回落")
	require.EqualValues(t, 5, after.Overview.TotalUV, "删除公寓后总 UV 不应回落")
	require.Len(t, after.Overview.BuildingRank, 1, "已删公寓不应出现在楼栋排行，其余公寓保留")
	require.Equal(t, f.A.ID, after.Overview.BuildingRank[0].BuildingID)
}
