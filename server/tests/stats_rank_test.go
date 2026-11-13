package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

// TestStatsRankExcludesDeletedBuildings 回归：迁移回填/历史数据可能给已删楼栋留下
// stat_daily 孤儿行；同时 detail_pv=0 的楼栋也不应入榜。排行查询必须把「存活且未软删
// + detail_pv>0」的过滤下推到聚合里再 LIMIT——否则先取 Top20 再过滤，孤儿行会把
// 活跃公寓挤出榜单甚至榜单为空。
func TestStatsRankExcludesDeletedBuildings(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	now := time.Now()
	todayKey := now.Format("2006-01-02")

	// 20 个已软删楼栋，各带高热度日行（模拟孤儿行挤占 Top20）
	for i := 0; i < 20; i++ {
		b := models.Building{Name: fmt.Sprintf("孤儿楼%02d", i), Status: "active"}
		require.NoError(t, app.DB.Create(&b).Error)
		require.NoError(t, app.DB.Delete(&b).Error) // 软删除
		require.NoError(t, app.DB.Create(&models.StatDaily{
			Day: todayKey, BuildingID: b.ID, PV: 1000, DetailPV: 1000, UpdatedAt: now,
		}).Error)
	}
	// 1 个存活楼栋，热度垫底
	active := models.Building{Name: "活跃楼", Status: "active"}
	require.NoError(t, app.DB.Create(&active).Error)
	require.NoError(t, app.DB.Create(&models.StatDaily{
		Day: todayKey, BuildingID: active.ID, PV: 1, DetailPV: 1, UpdatedAt: now,
	}).Error)
	// 1 个存活但无 detail 热度的楼栋（只有获电事件）：detail_pv=0 不应入榜
	lvOnly := models.Building{Name: "仅获电楼", Status: "active"}
	require.NoError(t, app.DB.Create(&lvOnly).Error)
	require.NoError(t, app.DB.Create(&models.StatDaily{
		Day: todayKey, BuildingID: lvOnly.ID, PV: 500, DetailPV: 0, LandlordView: 500, UpdatedAt: now,
	}).Error)

	r := app.Client.Do(http.MethodGet, "/api/admin/stats/overview", f.TokenRoot, nil)
	r.RequireOK(t)
	var data struct {
		Overview struct {
			BuildingRank []struct {
				BuildingID   uint   `json:"building_id"`
				BuildingName string `json:"building_name"`
			} `json:"building_rank"`
		} `json:"overview"`
	}
	r.Decode(t, &data)
	require.Len(t, data.Overview.BuildingRank, 1, "孤儿楼与 detail_pv=0 的楼栋不得入榜，活跃楼必须出现在 Top20 内")
	require.Equal(t, active.ID, data.Overview.BuildingRank[0].BuildingID)
	require.Equal(t, "活跃楼", data.Overview.BuildingRank[0].BuildingName)
}

// TestCleanupOrphanStatDaily 定时兜底：soft_delete_cleanup 任务应清掉已删楼栋的
// stat_daily 与 page_views 孤儿行，平台行（building_id=0）与存活楼栋不受影响。
func TestCleanupOrphanStatDaily(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	now := time.Now()
	todayKey := now.Format("2006-01-02")

	b := models.Building{Name: "将删楼", Status: "active"}
	require.NoError(t, app.DB.Create(&b).Error)
	require.NoError(t, app.DB.Delete(&b).Error) // 软删除，90 天内不会被 CleanupSoftDeleted 硬删
	require.NoError(t, app.DB.Create(&models.StatDaily{
		Day: todayKey, BuildingID: b.ID, PV: 9, DetailPV: 9, UpdatedAt: now,
	}).Error)
	require.NoError(t, app.DB.Create(&models.StatDaily{
		Day: todayKey, BuildingID: 0, PV: 9, DetailPV: 9, UpdatedAt: now,
	}).Error)
	// 存活楼栋的日行与明细必须保留；已删楼栋的 page_views 明细也应一并清掉
	active := models.Building{Name: "存活楼", Status: "active"}
	require.NoError(t, app.DB.Create(&active).Error)
	require.NoError(t, app.DB.Create(&models.StatDaily{
		Day: todayKey, BuildingID: active.ID, PV: 3, DetailPV: 3, UpdatedAt: now,
	}).Error)
	require.NoError(t, app.DB.Create(&models.PageView{
		PageType: "building_detail", BuildingID: b.ID, IP: "8.8.8.8", CreatedAt: now,
	}).Error)
	require.NoError(t, app.DB.Create(&models.PageView{
		PageType: "building_detail", BuildingID: active.ID, IP: "8.8.8.9", CreatedAt: now,
	}).Error)

	run := app.Client.Do(http.MethodPost, "/api/admin/system/jobs/soft_delete_cleanup/run", f.TokenRoot, nil)
	run.RequireOK(t)

	require.Zero(t, countWhere(t, app.DB, &models.StatDaily{}, "building_id = ?", b.ID), "孤儿楼栋行应被清理")
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.StatDaily{}, "building_id = 0"), "平台行必须保留")
	require.Zero(t, countWhere(t, app.DB, &models.PageView{}, "building_id = ?", b.ID), "已删楼栋的 page_views 明细应被清理")
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.PageView{}, "building_id = ?", active.ID), "存活楼栋的 page_views 明细必须保留")
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.StatDaily{}, "building_id = ?", active.ID), "存活楼栋的日行必须保留")
}
