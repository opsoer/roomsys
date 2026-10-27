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

func TestPlatform_BuildingCRUD(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	today := time.Now().Format("2006-01-02")
	nextYear := time.Now().AddDate(1, 0, 0).Format("2006-01-02")

	createBody := func(name string) map[string]any {
		return map[string]any{
			"name": name, "package": "full", "deposit": 3000,
			"contract_date": today, "expired_at": nextYear,
			"district": "南山区", "street": "科技园", "village": "测试花园", "building_no": "E座",
			"description": "平台测试楼栋",
			"landlords":   []any{map[string]any{"name": "平台房东", "phone": "13800000009"}},
		}
	}

	r := app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, createBody("平台测试公寓"))
	r.RequireOK(t)
	var created struct {
		Building models.Building `json:"building"`
	}
	r.Decode(t, &created)
	require.NotZero(t, created.Building.ID)
	require.Equal(t, "active", created.Building.Status)
	require.Equal(t, "full", created.Building.Package)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.BuildingDepositLog{}, "building_id = ? AND action = ?", created.Building.ID, "create"))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.BuildingRenewal{}, "building_id = ? AND action = ?", created.Building.ID, "join"))
	firstID := created.Building.ID

	dup := app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, createBody("平台测试公寓"))
	dup.RequireStatus(t, http.StatusConflict, 1006)
	require.Contains(t, string(dup.Raw), "suggested_name")

	neg := createBody("负数押金")
	neg["deposit"] = -1
	app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, neg).RequireError(t, http.StatusBadRequest, 400)

	badExpire := createBody("坏日期")
	badExpire["expired_at"] = "2026/13/01"
	app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, badExpire).RequireError(t, http.StatusBadRequest, 400)

	sameDay := createBody("同日到期")
	sameDay["expired_at"] = today
	app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, sameDay).RequireError(t, http.StatusBadRequest, 400)

	auto := createBody("自动到期")
	delete(auto, "expired_at")
	autoR := app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, auto)
	autoR.RequireOK(t)
	autoR.Decode(t, &created)
	parsed, err := time.Parse("2006-01-02", created.Building.ExpiredAt)
	require.NoError(t, err)
	require.Equal(t, time.Now().AddDate(1, 0, 0).Format("2006-01-02"), parsed.Format("2006-01-02"))

	vip := createBody("非法套餐")
	vip["package"] = "vip"
	vipR := app.Client.Do(http.MethodPost, "/api/admin/buildings", f.TokenRoot, vip)
	vipR.RequireOK(t)
	vipR.Decode(t, &created)
	require.Equal(t, "basic", created.Building.Package)

	// 列表筛选
	normal := app.Client.Do(http.MethodGet, "/api/admin/buildings?status=normal", f.TokenRoot, nil)
	normal.RequireOK(t)
	var list struct {
		Buildings []models.Building `json:"buildings"`
		Total     int               `json:"total"`
	}
	normal.Decode(t, &list)
	for _, b := range list.Buildings {
		require.True(t, b.ExpiredAt > time.Now().AddDate(0, 0, 30).Format("2006-01-02"))
	}
	expiring := app.Client.Do(http.MethodGet, "/api/admin/buildings?status=expiring", f.TokenRoot, nil)
	expiring.RequireOK(t)
	expiring.Decode(t, &list)
	require.NotEmpty(t, list.Buildings)
	for _, b := range list.Buildings {
		require.Equal(t, f.D.ID, b.ID)
	}
	unknown := app.Client.Do(http.MethodGet, "/api/admin/buildings?status=active", f.TokenRoot, nil)
	unknown.RequireOK(t)
	unknown.Decode(t, &list)
	require.GreaterOrEqual(t, list.Total, 4) // 当前实现未识别 active，等于不过滤

	kw := app.Client.Do(http.MethodGet, "/api/admin/buildings?keyword=测试公寓A", f.TokenRoot, nil)
	kw.RequireOK(t)
	kw.Decode(t, &list)
	require.GreaterOrEqual(t, list.Total, 1)

	// 更新名称/地址/房东
	upd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", firstID), f.TokenRoot, map[string]any{
		"name": "平台测试公寓改", "district": "福田区", "street": "梅林街道", "village": "测试村",
		"landlords": []any{map[string]any{"name": "新房东", "phone": "13800000010"}},
	})
	upd.RequireOK(t)
	got := getBuilding(t, app, firstID)
	require.Equal(t, "平台测试公寓改", got.Name)
	require.Equal(t, "福田区", got.District)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.BuildingLandlord{}, "building_id = ?", got.ID))

	dupUpd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", got.ID), f.TokenRoot, map[string]any{"name": f.B.Name})
	dupUpd.RequireStatus(t, http.StatusConflict, 1006)

	pkgUpd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", got.ID), f.TokenRoot, map[string]any{"package": "vip"})
	pkgUpd.RequireOK(t)
	require.Equal(t, "full", getBuilding(t, app, got.ID).Package)

	// 删除空楼栋成功；有合同楼栋 409
	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/buildings/%d", firstID), f.TokenRoot, nil)
	del.RequireOK(t)
	require.Zero(t, countWhere(t, app.DB, &models.Building{}, "id = ?", firstID))

	activeDel := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, nil)
	activeDel.RequireStatus(t, http.StatusConflict, 1003)
}

func TestPlatform_Package(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	up := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/package", f.C.ID), f.TokenRoot, map[string]any{"package": "full"})
	up.RequireOK(t)
	require.Equal(t, "full", getBuilding(t, app, f.C.ID).Package)
	app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenC, nil).RequireOK(t)

	down := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/package", f.C.ID), f.TokenRoot, map[string]any{"package": "basic"})
	down.RequireOK(t)
	require.Equal(t, "basic", getBuilding(t, app, f.C.ID).Package)
	app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenC, nil).RequireError(t, http.StatusForbidden, 403)

	bad := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/package", f.C.ID), f.TokenRoot, map[string]any{"package": "vip"})
	bad.RequireError(t, http.StatusBadRequest, 400)
}

func TestPlatform_RenewAndDeposit(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	newDate := time.Now().AddDate(1, 0, 0).Format("2006-01-02")

	r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.D.ID), f.TokenRoot, map[string]any{"expired_at": newDate})
	r.RequireOK(t)
	require.Equal(t, "active", getBuilding(t, app, f.D.ID).Status)
	require.Equal(t, newDate, getBuilding(t, app, f.D.ID).ExpiredAt)

	empty := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.D.ID), f.TokenRoot, map[string]any{"expired_at": ""})
	empty.RequireError(t, http.StatusBadRequest, 400)
	past := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.D.ID), f.TokenRoot, map[string]any{"expired_at": "2020-01-01"})
	past.RequireError(t, http.StatusBadRequest, 400)
	invalid := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.D.ID), f.TokenRoot, map[string]any{"expired_at": "2026/13/01"})
	invalid.RequireError(t, http.StatusBadRequest, 400)

	records := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/admin/buildings/%d/renewals", f.D.ID), f.TokenRoot, nil)
	records.RequireOK(t)
	require.Contains(t, string(records.Raw), "renew")

	// 押金
	adj := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/deposit", f.A.ID), f.TokenRoot, map[string]any{"amount": 1000, "reason": "补缴"})
	adj.RequireOK(t)
	var ad struct {
		Deposit float64 `json:"deposit"`
	}
	adj.Decode(t, &ad)
	require.InDelta(t, 6000, ad.Deposit, 0.01)

	adj2 := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/deposit", f.A.ID), f.TokenRoot, map[string]any{"amount": -2000, "reason": "扣罚"})
	adj2.RequireOK(t)
	adj2.Decode(t, &ad)
	require.InDelta(t, 4000, ad.Deposit, 0.01)

	zero := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/deposit", f.A.ID), f.TokenRoot, map[string]any{"amount": 0, "reason": "x"})
	zero.RequireError(t, http.StatusBadRequest, 400)
	noReason := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d/deposit", f.A.ID), f.TokenRoot, map[string]any{"amount": 100, "reason": "  "})
	noReason.RequireError(t, http.StatusBadRequest, 400)

	logs := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/admin/buildings/%d/deposit-logs", f.A.ID), f.TokenRoot, nil)
	logs.RequireOK(t)
	require.Contains(t, string(logs.Raw), "补缴")
	require.Contains(t, string(logs.Raw), "扣罚")

	missing := app.Client.Do(http.MethodGet, "/api/admin/buildings/999999/deposit-logs", f.TokenRoot, nil)
	missing.RequireError(t, http.StatusNotFound, 404)
}

func TestPlatform_AuditLogs(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 产生一条可过滤的日志
	createBill(t, app, f.TokenA, "income", "租金", 100, f.Today, "审计日志测试", nil)

	r := app.Client.Do(http.MethodGet, "/api/admin/system/audit-logs?resource=bill", f.TokenRoot, nil)
	r.RequireOK(t)
	var data struct {
		Logs  []models.AuditLog `json:"logs"`
		Total int64             `json:"total"`
		Page  int               `json:"page"`
		Limit int               `json:"limit"`
	}
	r.Decode(t, &data)
	require.GreaterOrEqual(t, data.Total, int64(1))
	require.NotEmpty(t, data.Logs)
	for _, l := range data.Logs {
		require.Equal(t, "bill", l.Resource)
	}

	byAction := app.Client.Do(http.MethodGet, "/api/admin/system/audit-logs?action=create", f.TokenRoot, nil)
	byAction.RequireOK(t)
	byAction.Decode(t, &data)
	require.GreaterOrEqual(t, data.Total, int64(1))

	page := app.Client.Do(http.MethodGet, "/api/admin/system/audit-logs?page=1&limit=10", f.TokenRoot, nil)
	page.RequireOK(t)
	page.Decode(t, &data)
	require.Equal(t, 10, data.Limit)

	clamped := app.Client.Do(http.MethodGet, "/api/admin/system/audit-logs?limit=999", f.TokenRoot, nil)
	clamped.RequireOK(t)
	clamped.Decode(t, &data)
	require.Equal(t, 50, data.Limit)

	app.Client.Do(http.MethodGet, "/api/admin/system/audit-logs", f.TokenA, nil).
		RequireError(t, http.StatusForbidden, 403)
}

func TestPlatform_Locations(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// level1
	l1 := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 1, "district": "测试新区"})
	l1.RequireOK(t)
	var c1 struct {
		Custom models.LocationCustom `json:"custom"`
	}
	l1.Decode(t, &c1)
	require.Equal(t, "测试新区", c1.Custom.Name)

	// level2
	l2 := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 2, "district": "测试新区", "name": "测试街道"})
	l2.RequireOK(t)
	var c2 struct {
		Custom models.LocationCustom `json:"custom"`
	}
	l2.Decode(t, &c2)

	// level3
	l3 := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 3, "district": "测试新区", "street": "测试街道", "name": "测试村"})
	l3.RequireOK(t)
	var c3 struct {
		Custom models.LocationCustom `json:"custom"`
	}
	l3.Decode(t, &c3)

	dup := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 3, "district": "测试新区", "street": "测试街道", "name": "测试村"})
	dup.RequireError(t, http.StatusBadRequest, 400)

	badLevel := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 4, "district": "x"})
	badLevel.RequireError(t, http.StatusBadRequest, 400)

	// 被公寓引用的位置禁止删除
	used := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 3, "district": "南山区", "street": "科技园", "name": "测试花园"})
	used.RequireOK(t)
	var cu struct {
		Custom models.LocationCustom `json:"custom"`
	}
	used.Decode(t, &cu)
	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/locations/custom/%d", cu.Custom.ID), f.TokenRoot, nil).
		RequireError(t, http.StatusBadRequest, 400)

	// 上级有子条目禁止删除
	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/locations/custom/%d", c1.Custom.ID), f.TokenRoot, nil).
		RequireError(t, http.StatusBadRequest, 400)

	// 未被引用的 level3 可删除
	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/locations/custom/%d", c3.Custom.ID), f.TokenRoot, nil).RequireOK(t)

	// 改名 dry_run
	dry := app.Client.Do(http.MethodPost, "/api/admin/locations/rename", f.TokenRoot, map[string]any{
		"level": 3, "district": "南山区", "street": "科技园", "old_name": "测试花园", "new_name": "改名花园", "dry_run": true,
	})
	dry.RequireOK(t)
	var dr struct {
		Affected int64 `json:"affected_buildings"`
	}
	dry.Decode(t, &dr)
	require.EqualValues(t, 2, dr.Affected)

	same := app.Client.Do(http.MethodPost, "/api/admin/locations/rename", f.TokenRoot, map[string]any{
		"level": 3, "district": "南山区", "street": "科技园", "old_name": "测试花园", "new_name": "测试花园",
	})
	same.RequireError(t, http.StatusBadRequest, 400)

	conflict := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{"level": 3, "district": "南山区", "street": "科技园", "name": "冲突花园"})
	conflict.RequireOK(t)
	app.Client.Do(http.MethodPost, "/api/admin/locations/rename", f.TokenRoot, map[string]any{
		"level": 3, "district": "南山区", "street": "科技园", "old_name": "测试花园", "new_name": "冲突花园",
	}).RequireError(t, http.StatusBadRequest, 400)

	real := app.Client.Do(http.MethodPost, "/api/admin/locations/rename", f.TokenRoot, map[string]any{
		"level": 3, "district": "南山区", "street": "科技园", "old_name": "测试花园", "new_name": "改名花园",
	})
	real.RequireOK(t)
	require.Equal(t, "改名花园", getBuilding(t, app, f.A.ID).Village)
	require.Equal(t, "改名花园", getBuilding(t, app, f.B.ID).Village)

	// 链式改名：改名花园 -> 最终花园，旧映射 new_name 跟随
	app.Client.Do(http.MethodPost, "/api/admin/locations/rename", f.TokenRoot, map[string]any{
		"level": 3, "district": "南山区", "street": "科技园", "old_name": "改名花园", "new_name": "最终花园",
	}).RequireOK(t)
	var rename models.LocationRename
	require.NoError(t, app.DB.Where("level = ? AND old_name = ?", 3, "测试花园").First(&rename).Error)
	require.Equal(t, "最终花园", rename.NewName)
}

func TestPlatform_Settings(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	missing := app.Client.Do(http.MethodGet, "/api/admin/settings/not_exist_key", f.TokenRoot, nil)
	missing.RequireOK(t)
	var data struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	missing.Decode(t, &data)
	require.Equal(t, "not_exist_key", data.Key)
	require.Empty(t, data.Value)

	// 写入后读取应返回写入的值（BUG-012 修复后）
	upd := app.Client.Do(http.MethodPut, "/api/admin/settings/test_key", f.TokenRoot, map[string]any{"value": "hello"})
	upd.RequireOK(t)
	get := app.Client.Do(http.MethodGet, "/api/admin/settings/test_key", f.TokenRoot, nil)
	get.RequireOK(t)
	get.Decode(t, &data)
	require.Equal(t, "hello", data.Value)

	bad := app.Client.DoWithHeaders(http.MethodPut, "/api/admin/settings/test_key", f.TokenRoot, nil, map[string]string{"Content-Type": "application/json"})
	bad.RequireError(t, http.StatusBadRequest, 400)
}

func TestPlatform_Stats(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	overview := app.Client.Do(http.MethodGet, "/api/admin/stats/overview", f.TokenRoot, nil)
	overview.RequireOK(t)
	require.Contains(t, string(overview.Raw), "overview")

	trend := app.Client.Do(http.MethodGet, "/api/admin/stats/trend?days=7", f.TokenRoot, nil)
	trend.RequireOK(t)
	require.Contains(t, string(trend.Raw), "trend")
	app.Client.Do(http.MethodGet, "/api/admin/stats/trend?days=366", f.TokenRoot, nil).RequireOK(t)
	app.Client.Do(http.MethodGet, "/api/admin/stats/trend?days=abc", f.TokenRoot, nil).RequireOK(t)

	price := app.Client.Do(http.MethodGet, "/api/admin/stats/price-reference", f.TokenRoot, nil)
	price.RequireOK(t)
	require.Contains(t, string(price.Raw), "price_reference")

	detail := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/admin/stats/building/%d", f.A.ID), f.TokenRoot, nil)
	detail.RequireOK(t)
	app.Client.Do(http.MethodGet, "/api/admin/stats/building/999999", f.TokenRoot, nil).
		RequireError(t, http.StatusNotFound, 404)

	app.Client.Do(http.MethodGet, "/api/admin/stats/overview", f.TokenA, nil).
		RequireError(t, http.StatusForbidden, 403)
}

func TestPlatform_Recruit(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	sub := app.Client.Do(http.MethodPost, "/api/recruit/submit", "", map[string]any{"phone": "13877778888", "address": "测试地址"})
	sub.RequireOK(t)

	list := app.Client.Do(http.MethodGet, "/api/admin/recruit/list", f.TokenRoot, nil)
	list.RequireOK(t)
	var data struct {
		Submissions []models.RecruitSubmission `json:"submissions"`
	}
	list.Decode(t, &data)
	require.NotEmpty(t, data.Submissions)
	for _, s := range data.Submissions {
		require.NotEmpty(t, s.Phone)
	}

	var subModel models.RecruitSubmission
	require.NoError(t, app.DB.Where("phone = ?", "13877778888").First(&subModel).Error)
	proc := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/recruit/process/%d", subModel.ID), f.TokenRoot, nil)
	proc.RequireOK(t)
	require.NoError(t, app.DB.First(&subModel, subModel.ID).Error)
	require.Equal(t, "processed", subModel.Status)

	// 重复处理当前实现仍成功
	app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/recruit/process/%d", subModel.ID), f.TokenRoot, nil).RequireOK(t)
}
func TestPlatform_BuildingSelfServiceAndLocationsList(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	upd := app.Client.Do(http.MethodPut, "/api/building/info", f.TokenA, map[string]any{
		"name": "A栋改名", "description": "新简介", "cover_image": "buildings/a/cover.png",
	})
	upd.RequireOK(t)
	got := getBuilding(t, app, f.A.ID)
	require.Equal(t, "A栋改名", got.Name)
	require.Equal(t, "新简介", got.Description)
	require.Equal(t, "buildings/a/cover.png", got.CoverImage)

	require.Contains(t, string(app.Client.Do(http.MethodGet, "/api/building/stats", f.TokenA, nil).RequireOK(t).Raw), "stats")
	require.Contains(t, string(app.Client.Do(http.MethodGet, "/api/building/stats/pv", f.TokenA, nil).RequireOK(t).Raw), "stats")
	require.Contains(t, string(app.Client.Do(http.MethodGet, "/api/building/stats/trend?days=7", f.TokenA, nil).RequireOK(t).Raw), "trend")

	loc := app.Client.Do(http.MethodGet, "/api/admin/locations/manage", f.TokenRoot, nil)
	loc.RequireOK(t)
	require.Contains(t, string(loc.Raw), "customs")
	require.Contains(t, string(loc.Raw), "renames")

	view := app.Client.Do(http.MethodPost, "/api/stats/landlord-view", "", map[string]any{"building_id": f.A.ID})
	view.RequireOK(t)
}
