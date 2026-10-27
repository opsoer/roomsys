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

func TestScenario_PlatformOnboardingToPublic(t *testing.T) {
	app := testkit.NewApp(t)

	rootLogin := login(t, app.Client, "root", "root")
	rootLogin.RequireOK(t)
	var rootData struct {
		Token string `json:"token"`
	}
	rootLogin.Decode(t, &rootData)

	today := time.Now().Format("2006-01-02")
	expire := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	createB := app.Client.Do(http.MethodPost, "/api/admin/buildings", rootData.Token, map[string]any{
		"name": "端到端公寓", "package": "full", "deposit": 1000,
		"contract_date": today, "expired_at": expire,
		"district": "南山区", "street": "科技园", "village": "端到端花园",
		"landlords": []any{map[string]any{"name": "端到端房东", "phone": "13800000099"}},
	})
	createB.RequireOK(t)
	var bd struct {
		Building models.Building `json:"building"`
	}
	createB.Decode(t, &bd)
	bid := bd.Building.ID

	createAdmin := app.Client.Do(http.MethodPost, "/api/admin/auth/create-building-admin", rootData.Token, map[string]any{
		"username": "e2eadmin", "password": "Test12345", "building_id": bid,
	})
	createAdmin.RequireOK(t)

	lr := login(t, app.Client, "e2eadmin", "Test12345")
	lr.RequireOK(t)
	var ld struct {
		Token string `json:"token"`
	}
	lr.Decode(t, &ld)

	room := createRoom(t, app, ld.Token, "E2E101", 2000, 200)

	// 公开端可见空房
	pub := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/buildings/%d/rooms", bid), "", nil)
	pub.RequireOK(t)
	var rooms roomListResp
	pub.Decode(t, &rooms)
	require.Contains(t, roomIDs(rooms.Rooms), room.ID)

	// 出租到 10 天内到期，公开端仍可见
	start := testkit.MonthDay(time.Now().Format("2006-01"), 1)
	end := time.Now().AddDate(0, 0, 10).Format("2006-01-02")
	rent := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", room.ID), ld.Token, rentBody(room.ID, start, end))
	rent.RequireOK(t)

	pub2 := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/buildings/%d/rooms?status=expiring", bid), "", nil)
	pub2.RequireOK(t)
	pub2.Decode(t, &rooms)
	require.Contains(t, roomIDs(rooms.Rooms), room.ID)

	bills := app.Client.Do(http.MethodGet, "/api/building/bills", ld.Token, nil)
	bills.RequireOK(t)
	require.Contains(t, string(bills.Raw), "E2E101")
}

func TestScenario_RentToSettleAndPredict(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	before := app.Client.Do(http.MethodGet, "/api/building/bills/stats?month="+f.ThisMonth, f.TokenA, nil)
	before.RequireOK(t)
	var beforeStats struct {
		TotalIncome float64 `json:"total_income"`
		NetProfit   float64 `json:"net_profit"`
	}
	before.Decode(t, &beforeStats)

	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)
	rent := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, rentBody(f.Rooms["A101"].ID, start, end))
	rent.RequireOK(t)

	after := app.Client.Do(http.MethodGet, "/api/building/bills/stats?month="+f.ThisMonth, f.TokenA, nil)
	after.RequireOK(t)
	var afterStats struct {
		TotalIncome float64 `json:"total_income"`
		NetProfit   float64 `json:"net_profit"`
	}
	after.Decode(t, &afterStats)
	// 租金 2200 + 押金 2000
	require.InDelta(t, 4200, afterStats.TotalIncome-beforeStats.TotalIncome, 0.01)

	predict := app.Client.Do(http.MethodGet, "/api/building/dividends/predict", f.TokenA, nil)
	predict.RequireOK(t)
	var pd struct {
		Predictions []struct {
			Rent float64 `json:"rent"`
		} `json:"predictions"`
	}
	predict.Decode(t, &pd)
	require.InDelta(t, 8800, pd.Predictions[0].Rent, 0.01)

	settle := app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{"month": f.ThisMonth})
	settle.RequireOK(t)

	var divs []models.Dividend
	require.NoError(t, app.DB.Where("building_id = ? AND settle_month = ?", f.A.ID, f.ThisMonth).Find(&divs).Error)
	require.Len(t, divs, 2)
	sum := 0.0
	for _, d := range divs {
		sum += d.DividendAmount
	}
	require.InDelta(t, afterStats.NetProfit, sum, 0.01)
}

func TestScenario_ReserveCancelConfirm(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)

	reserve := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, reserveBody(f.Rooms["A101"].ID, start, end, 500))
	reserve.RequireOK(t)
	require.Equal(t, "reserved", getRoom(t, app, f.Rooms["A101"].ID).Status)

	cancel := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, map[string]any{
		"status": "vacant", "refunded_deposit": 0,
	})
	cancel.RequireOK(t)
	require.Equal(t, "vacant", getRoom(t, app, f.Rooms["A101"].ID).Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A101"].ID, "定金违约"))

	// 再次预订并确认签约
	reserve2 := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, reserveBody(f.Rooms["A101"].ID, start, end, 500))
	reserve2.RequireOK(t)
	confirm := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, rentBody(f.Rooms["A101"].ID, start, end))
	confirm.RequireOK(t)

	require.Equal(t, "rented", getRoom(t, app, f.Rooms["A101"].ID).Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A101"].ID, "active"))
	require.EqualValues(t, 0, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A101"].ID, "reserved"))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A101"].ID, "租金"))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A101"].ID, "押金"))
}

func TestScenario_VacateAndConfirmFutureReservation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	vacate := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A105"].ID), f.TokenA, map[string]any{
		"status": "vacant", "refunded_deposit": 2200,
	})
	vacate.RequireOK(t)
	require.Equal(t, "reserved", getRoom(t, app, f.Rooms["A105"].ID).Status)
	require.Equal(t, "ended", getContract(t, app, f.ContractA105.ID).Status)
	require.Equal(t, "reserved", getContract(t, app, f.ContractA105Future.ID).Status)

	// 确认未来预订签约
	confirm := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A105"].ID), f.TokenA, rentBody(
		f.Rooms["A105"].ID, f.ContractA105Future.StartDate, f.ContractA105Future.EndDate))
	confirm.RequireOK(t)
	require.Equal(t, "rented", getRoom(t, app, f.Rooms["A105"].ID).Status)
	require.Equal(t, "active", getContract(t, app, f.ContractA105Future.ID).Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A105"].ID, "active"))
}

func TestScenario_BuildingExpiryRenew(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	app.SetTime(t, time.Now().AddDate(0, 0, 11))
	app.Client.Do(http.MethodPost, "/api/admin/system/jobs/building_expiry/run", f.TokenRoot, nil).RequireOK(t)
	require.Equal(t, "hidden", getBuilding(t, app, f.D.ID).Status)
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.D.ID), "", nil).RequireError(t, http.StatusNotFound, 404)
	login(t, app.Client, "dadmin", "Test12345").RequireError(t, http.StatusForbidden, 403)

	app.ResetTime()
	renew := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.D.ID), f.TokenRoot, map[string]any{
		"expired_at": time.Now().AddDate(1, 0, 0).Format("2006-01-02"),
	})
	renew.RequireOK(t)
	require.Equal(t, "active", getBuilding(t, app, f.D.ID).Status)
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.D.ID), "", nil).RequireOK(t)
	login(t, app.Client, "dadmin", "Test12345").RequireOK(t)
}

func TestScenario_ImportAndBatchPricing(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	item := func(number string) map[string]any {
		return map[string]any{
			"room_number": number, "floor": "7", "layout": "单间",
			"rent_price": 1800, "deposit_months": 1, "management_fee": 180,
			"electricity_unit_price": 1.2, "water_unit_price": 4,
			"tenant_name": "批量租客", "tenant_phone": "13933334444",
			"deposit": 1800, "start_date": f.Today, "end_date": "2099-12-31",
		}
	}
	imp := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{
		"rooms": []any{item("P001"), item("P002")},
	})
	imp.RequireOK(t)

	var rooms []models.Room
	require.NoError(t, app.DB.Where("room_number IN ?", []string{"P001", "P002"}).Find(&rooms).Error)
	require.Len(t, rooms, 2)

	batch := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{
		"room_ids": []uint{rooms[0].ID, rooms[1].ID}, "rent_delta": 100.0, "management_fee": 250.0,
	})
	batch.RequireOK(t)
	for _, r := range rooms {
		var got models.Room
		require.NoError(t, app.DB.First(&got, r.ID).Error)
		require.Equal(t, 1900.0, *got.RentPrice)
		require.Equal(t, 250.0, *got.ManagementFee)
	}
}
