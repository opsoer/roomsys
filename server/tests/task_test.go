package tests

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"rental-server/models"
	"rental-server/tests/testkit"
)

func TestTask_ListAndProcess(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	list := app.Client.Do(http.MethodGet, "/api/building/tasks?status=pending", f.TokenA, nil)
	list.RequireOK(t)
	var data struct {
		Tasks []models.Task `json:"tasks"`
		Total int           `json:"total"`
	}
	list.Decode(t, &data)
	require.Equal(t, 1, data.Total)
	require.Equal(t, "expired_room", data.Tasks[0].Type)

	r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", f.TaskA104.ID), f.TokenA, map[string]any{
		"refunded_deposit": 1800,
	})
	r.RequireOK(t)
	require.Equal(t, "vacant", getRoom(t, app, f.Rooms["A104"].ID).Status)
	require.Equal(t, "ended", getContract(t, app, f.ContractA104.ID).Status)
	require.Equal(t, "completed", getTask(t, app, f.TaskA104.ID).Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A104"].ID, "押金退还"))

	again := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", f.TaskA104.ID), f.TokenA, map[string]any{"refunded_deposit": 100})
	again.RequireError(t, http.StatusBadRequest, 400).
		RequireMessageContains(t, "已处理")
}

func TestTask_ProcessRules(t *testing.T) {
	t.Run("zero refund no bill", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", f.TaskA104.ID), f.TokenA, map[string]any{"refunded_deposit": 0})
		r.RequireOK(t)
		require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A104"].ID, "押金退还"))
	})

	t.Run("negative refund", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", f.TaskA104.ID), f.TokenA, map[string]any{"refunded_deposit": -1})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("unsupported type", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		task := models.Task{BuildingID: &f.A.ID, Title: "预订超时", Type: "reserved_overdue", Status: "pending", RoomID: uptr(f.Rooms["A103"].ID), Scope: "building"}
		require.NoError(t, app.DB.Create(&task).Error)
		r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", task.ID), f.TokenA, map[string]any{"refunded_deposit": 0})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("cross building", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		task := models.Task{BuildingID: &f.B.ID, Title: "B 待办", Type: "expired_room", Status: "pending", RoomID: uptr(f.Rooms["B202"].ID), Scope: "building"}
		require.NoError(t, app.DB.Create(&task).Error)
		r := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", task.ID), f.TokenA, map[string]any{"refunded_deposit": 0})
		r.RequireError(t, http.StatusNotFound, 404)
	})
}

func TestTask_CompleteAndDelete(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	task := models.Task{BuildingID: &f.A.ID, Title: "普通待办", Type: "recruit", Status: "pending", Scope: "building"}
	require.NoError(t, app.DB.Create(&task).Error)

	done := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/tasks/%d/complete", task.ID), f.TokenA, nil)
	done.RequireOK(t)
	require.Equal(t, "completed", getTask(t, app, task.ID).Status)

	delTask := models.Task{BuildingID: &f.A.ID, Title: "删除待办", Type: "recruit", Status: "pending", Scope: "building"}
	require.NoError(t, app.DB.Create(&delTask).Error)
	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/tasks/%d", delTask.ID), f.TokenA, nil)
	del.RequireOK(t)
	require.Zero(t, countWhere(t, app.DB, &models.Task{}, "id = ?", delTask.ID))

	cross := models.Task{BuildingID: &f.B.ID, Title: "B", Type: "recruit", Status: "pending", Scope: "building"}
	require.NoError(t, app.DB.Create(&cross).Error)
	app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/tasks/%d/complete", cross.ID), f.TokenA, nil).
		RequireError(t, http.StatusNotFound, 404)
	app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/tasks/%d", cross.ID), f.TokenA, nil).
		RequireError(t, http.StatusNotFound, 404)
}

func TestTask_Platform(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	sub := app.Client.Do(http.MethodPost, "/api/recruit/submit", "", map[string]any{"phone": "13877778888", "address": "测试地址"})
	sub.RequireOK(t)

	var task models.Task
	require.NoError(t, app.DB.Where("type = ? AND scope = ?", "recruit", "platform").First(&task).Error)

	list := app.Client.Do(http.MethodGet, "/api/admin/tasks?status=pending", f.TokenRoot, nil)
	list.RequireOK(t)
	var data struct {
		Tasks []models.Task `json:"tasks"`
		Total int           `json:"total"`
	}
	list.Decode(t, &data)
	require.GreaterOrEqual(t, data.Total, 1)

	count := app.Client.Do(http.MethodGet, "/api/admin/tasks/count", f.TokenRoot, nil)
	count.RequireOK(t)
	var cd struct {
		Count int `json:"count"`
	}
	count.Decode(t, &cd)
	require.GreaterOrEqual(t, cd.Count, 1)

	done := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/tasks/%d/process", task.ID), f.TokenRoot, nil)
	done.RequireOK(t)
	require.Equal(t, "completed", getTask(t, app, task.ID).Status)

	again := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/tasks/%d/process", task.ID), f.TokenRoot, nil)
	again.RequireError(t, http.StatusBadRequest, 400)

	// building scope 不能用平台接口处理
	bTask := models.Task{BuildingID: &f.A.ID, Title: "A", Type: "recruit", Status: "pending", Scope: "building"}
	require.NoError(t, app.DB.Create(&bTask).Error)
	app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/tasks/%d/process", bTask.ID), f.TokenRoot, nil).
		RequireError(t, http.StatusNotFound, 404)
}

func TestTask_TimeEndpoints(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	get := app.Client.Do(http.MethodGet, "/api/admin/system/time", f.TokenRoot, nil)
	get.RequireOK(t)
	var td struct {
		Offset int64 `json:"offset_seconds"`
	}
	get.Decode(t, &td)
	require.Zero(t, td.Offset)

	offset := app.Client.Do(http.MethodPost, "/api/admin/system/time", f.TokenRoot, map[string]any{"offset_seconds": 86400})
	offset.RequireOK(t)
	offset.Decode(t, &td)
	require.EqualValues(t, 86400, td.Offset)
	app.Client.Do(http.MethodPost, "/api/admin/system/time", f.TokenRoot, map[string]any{"offset_seconds": 0}).RequireOK(t)

	bad := app.Client.Do(http.MethodPost, "/api/admin/system/time", f.TokenRoot, map[string]any{"target_time": "2026/10/01 03:00:00"})
	bad.RequireError(t, http.StatusBadRequest, 400)
	missing := app.Client.Do(http.MethodPost, "/api/admin/system/time", f.TokenRoot, map[string]any{})
	missing.RequireError(t, http.StatusBadRequest, 400)

	// 设置目标时间会同步触发 contract_expiry
	target := time.Now().AddDate(0, 0, 31).Format("2006-01-02") + " 03:00:00"
	set := app.Client.Do(http.MethodPost, "/api/admin/system/time", f.TokenRoot, map[string]any{"target_time": target})
	set.RequireOK(t)
	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.Task{}, "room_id = ? AND type = ?", f.Rooms["A105"].ID, "expired_room"), int64(1))
	app.Client.Do(http.MethodPost, "/api/admin/system/time", f.TokenRoot, map[string]any{"offset_seconds": 0}).RequireOK(t)
}

func TestTask_Jobs(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	list := app.Client.Do(http.MethodGet, "/api/admin/system/jobs", f.TokenRoot, nil)
	list.RequireOK(t)
	var data struct {
		Jobs []struct {
			Name     string `json:"name"`
			Title    string `json:"title"`
			Enabled  bool   `json:"enabled"`
			Schedule string `json:"schedule"`
		} `json:"jobs"`
	}
	list.Decode(t, &data)
	require.Len(t, data.Jobs, 6)
	names := map[string]bool{}
	for _, j := range data.Jobs {
		names[j.Name] = true
		require.NotEmpty(t, j.Schedule)
	}
	for _, name := range []string{"rent_bills", "building_expiry", "contract_expiry", "soft_delete_cleanup", "token_cleanup", "db_backup"} {
		require.True(t, names[name], "missing job %s", name)
	}

	run := app.Client.Do(http.MethodPost, "/api/admin/system/jobs/rent_bills/run", f.TokenRoot, nil)
	run.RequireOK(t)
	var rd struct {
		Result string `json:"result"`
	}
	run.Decode(t, &rd)
	require.NotEmpty(t, rd.Result)

	unknown := app.Client.Do(http.MethodPost, "/api/admin/system/jobs/not_exist/run", f.TokenRoot, nil)
	unknown.RequireError(t, http.StatusBadRequest, 400)

	all := app.Client.Do(http.MethodPost, "/api/admin/system/run-tasks", f.TokenRoot, nil)
	all.RequireOK(t)
	var ad struct {
		Results []string `json:"results"`
	}
	all.Decode(t, &ad)
	require.Len(t, ad.Results, 6)
}

func TestTask_ContractExpiry(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	app.SetTime(t, time.Now().AddDate(0, 0, 31))

	run := app.Client.Do(http.MethodPost, "/api/admin/system/jobs/contract_expiry/run", f.TokenRoot, nil)
	run.RequireOK(t)

	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.Task{}, "room_id = ? AND type = ?", f.Rooms["A105"].ID, "expired_room"), int64(1))
	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.Task{}, "room_id = ? AND type = ?", f.Rooms["A103"].ID, "reserved_overdue"), int64(1))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Task{}, "room_id = ? AND type = ? AND status = ?", f.Rooms["A104"].ID, "expired_room", "pending"))

	// 幂等
	app.Client.Do(http.MethodPost, "/api/admin/system/jobs/contract_expiry/run", f.TokenRoot, nil).RequireOK(t)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Task{}, "room_id = ? AND type = ? AND status = ?", f.Rooms["A105"].ID, "expired_room", "pending"))
}

func TestTask_BuildingExpiryAndRenew(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 30 天内 -> expiring + 双端待办
	app.SetTime(t, time.Now().AddDate(0, 0, 1))
	app.Client.Do(http.MethodPost, "/api/admin/system/jobs/building_expiry/run", f.TokenRoot, nil).RequireOK(t)
	require.Equal(t, "expiring", getBuilding(t, app, f.D.ID).Status)
	require.EqualValues(t, 2, countWhere(t, app.DB, &models.Task{}, "building_id = ? AND type = ? AND status = ?", f.D.ID, "building_expiring", "pending"))

	// 已到期 -> hidden + 双端待办 + 公开端 404 + 登录 403
	app.SetTime(t, time.Now().AddDate(0, 0, 11))
	app.Client.Do(http.MethodPost, "/api/admin/system/jobs/building_expiry/run", f.TokenRoot, nil).RequireOK(t)
	require.Equal(t, "hidden", getBuilding(t, app, f.D.ID).Status)
	require.GreaterOrEqual(t, countWhere(t, app.DB, &models.Task{}, "building_id = ? AND type = ? AND status = ?", f.D.ID, "building_expired", "pending"), int64(2))
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.D.ID), "", nil).RequireError(t, http.StatusNotFound, 404)
	login(t, app.Client, "dadmin", "Test12345").RequireError(t, http.StatusForbidden, 403)

	// 重置时间后续约恢复
	app.ResetTime()
	renew := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.D.ID), f.TokenRoot, map[string]any{
		"expired_at": time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
	})
	renew.RequireOK(t)
	require.Equal(t, "active", getBuilding(t, app, f.D.ID).Status)
	require.Equal(t, int64(0), countWhere(t, app.DB, &models.Task{}, "building_id = ? AND type IN ? AND status = ?", f.D.ID, []string{"building_expired", "building_expiring"}, "pending"))
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.D.ID), "", nil).RequireOK(t)
}

func TestTask_RentBills(t *testing.T) {
	t.Run("creates missing current bill and is idempotent", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		// 让 A104 覆盖整月，房间管理费 300、合同管理费 200，验证取房间口径
		start := testkit.MonthDay(f.ThisMonth, 1)
		end := testkit.MonthLastDay(f.ThisMonth)
		require.NoError(t, app.DB.Model(&models.RentalContract{}).Where("id = ?", f.ContractA104.ID).
			Updates(map[string]any{"start_date": start, "end_date": end, "management_fee": 200.0}).Error)
		require.NoError(t, app.DB.Model(&models.Room{}).Where("id = ?", f.Rooms["A104"].ID).
			Update("management_fee", 300.0).Error)

		app.Client.Do(http.MethodPost, "/api/admin/system/jobs/rent_bills/run", f.TokenRoot, nil).RequireOK(t)
		var bill models.Bill
		require.NoError(t, app.DB.Where("room_id = ? AND subtype = ?", f.Rooms["A104"].ID, "租金").First(&bill).Error)
		require.InDelta(t, 2100.0, bill.Amount, 0.01)
		require.Equal(t, f.Today, bill.BillDate)
		require.Contains(t, bill.Description, "租金：1800.00元")
		require.Contains(t, bill.Description, "管理费：300.00元")

		app.Client.Do(http.MethodPost, "/api/admin/system/jobs/rent_bills/run", f.TokenRoot, nil).RequireOK(t)
		require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A104"].ID, "租金"))
	})

	t.Run("skips ended contract", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		require.NoError(t, app.DB.Model(&models.RentalContract{}).Where("id = ?", f.ContractA104.ID).Update("status", "ended").Error)
		app.Client.Do(http.MethodPost, "/api/admin/system/jobs/rent_bills/run", f.TokenRoot, nil).RequireOK(t)
		require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A104"].ID, "租金"))
	})
}

func TestTask_Cleanup(t *testing.T) {
	app := testkit.NewApp(t)

	old := time.Now().AddDate(0, 0, -100)
	recent := time.Now().AddDate(0, 0, -1)

	oldBill := models.Bill{BillNo: "OLD-1", Type: "income", Subtype: "其他", Amount: 1, BuildingID: 1, BillDate: "2020-01-01", PaidStatus: "paid", DeletedAt: gorm.DeletedAt{Time: old, Valid: true}}
	recentBill := models.Bill{BillNo: "RECENT-1", Type: "income", Subtype: "其他", Amount: 1, BuildingID: 1, BillDate: "2020-01-01", PaidStatus: "paid", DeletedAt: gorm.DeletedAt{Time: recent, Valid: true}}
	require.NoError(t, app.DB.Unscoped().Create(&oldBill).Error)
	require.NoError(t, app.DB.Unscoped().Create(&recentBill).Error)

	oldPV := models.PageView{PageType: "test", IP: "1.1.1.1", CreatedAt: old}
	recentPV := models.PageView{PageType: "test", IP: "1.1.1.2", CreatedAt: recent}
	require.NoError(t, app.DB.Create(&oldPV).Error)
	require.NoError(t, app.DB.Create(&recentPV).Error)

	oldAuditTime := time.Now().AddDate(0, 0, -200)
	oldAudit := models.AuditLog{UserID: 1, Username: "u", Action: "test", Resource: "test", CreatedAt: oldAuditTime}
	recentAudit := models.AuditLog{UserID: 1, Username: "u", Action: "test", Resource: "test", CreatedAt: recent}
	require.NoError(t, app.DB.Create(&oldAudit).Error)
	require.NoError(t, app.DB.Create(&recentAudit).Error)

	root := login(t, app.Client, "root", "root")
	root.RequireOK(t)
	var ld struct {
		Token string `json:"token"`
	}
	root.Decode(t, &ld)
	run := app.Client.Do(http.MethodPost, "/api/admin/system/jobs/soft_delete_cleanup/run", ld.Token, nil)
	run.RequireOK(t)

	require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "bill_no = ?", "OLD-1"))
	require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "bill_no = ?", "RECENT-1"))
	require.EqualValues(t, 1, countWhere(t, app.DB.Unscoped(), &models.Bill{}, "bill_no = ?", "RECENT-1"))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.PageView{}, "ip = ?", "1.1.1.2"))
	require.Zero(t, countWhere(t, app.DB, &models.PageView{}, "ip = ?", "1.1.1.1"))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.AuditLog{}, "username = ?", "u"))
}

func getBuilding(t *testing.T, app *testkit.App, id uint) models.Building {
	t.Helper()
	var b models.Building
	require.NoError(t, app.DB.First(&b, id).Error)
	return b
}
