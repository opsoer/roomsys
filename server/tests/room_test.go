package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

func createRoom(t *testing.T, app *testkit.App, token, number string, rent, mgmt float64) models.Room {
	t.Helper()
	body := map[string]any{
		"room_number": number, "floor": "9", "layout": "单间",
		"rent_price": rent, "deposit_months": 1, "management_fee": mgmt,
		"electricity_unit_price": 1.5, "water_unit_price": 4.5,
	}
	r := app.Client.Do(http.MethodPost, "/api/building/rooms", token, body)
	r.RequireOK(t)
	var data struct {
		Room models.Room `json:"room"`
	}
	r.Decode(t, &data)
	require.NotZero(t, data.Room.ID)
	return data.Room
}

func rentBody(roomID uint, start, end string) map[string]any {
	return map[string]any{
		"status": "rented", "tenant_name": "测试租客", "tenant_phone": "13911112222",
		"rent_price": 2000, "management_fee": 200, "deposit": 2000,
		"start_date": start, "end_date": end,
	}
}

func reserveBody(roomID uint, start, end string, earnest float64) map[string]any {
	return map[string]any{
		"status": "reserved", "tenant_name": "预订客", "tenant_phone": "13922223333",
		"rent_price": 2000, "management_fee": 200, "deposit": 2000,
		"earnest_money": earnest, "start_date": start, "end_date": end,
	}
}

func TestRoom_CreateValidation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	created := createRoom(t, app, f.TokenA, "T001", 1500, 150)
	require.Equal(t, "vacant", getRoom(t, app, created.ID).Status)
	require.Equal(t, f.A.ID, created.BuildingID)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing room_number", map[string]any{"floor": "1", "layout": "单间", "rent_price": 1, "deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1}},
		{"missing floor", map[string]any{"room_number": "X1", "layout": "单间", "rent_price": 1, "deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1}},
		{"duplicate", map[string]any{"room_number": "A101", "floor": "1", "layout": "单间", "rent_price": 1, "deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1}},
		{"zero rent", map[string]any{"room_number": "X2", "floor": "1", "layout": "单间", "rent_price": 0, "deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1}},
		{"negative rent", map[string]any{"room_number": "X3", "floor": "1", "layout": "单间", "rent_price": -1, "deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1}},
		{"negative mgmt", map[string]any{"room_number": "X4", "floor": "1", "layout": "单间", "rent_price": 1, "deposit_months": 1, "management_fee": -1, "electricity_unit_price": 1, "water_unit_price": 1}},
		{"deposit months 4", map[string]any{"room_number": "X5", "floor": "1", "layout": "单间", "rent_price": 1, "deposit_months": 4, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := app.Client.Do(http.MethodPost, "/api/building/rooms", f.TokenA, tc.body)
			require.Contains(t, []int{400, 409}, r.Status, "raw=%s", string(r.Raw))
		})
	}

	// 跨栋复用媒体源
	cross := app.Client.Do(http.MethodPost, "/api/building/rooms", f.TokenA, map[string]any{
		"room_number": "X6", "floor": "1", "layout": "单间", "rent_price": 1,
		"deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1,
		"copy_from_room_id": f.Rooms["B201"].ID,
	})
	cross.RequireError(t, http.StatusBadRequest, 400)
}

func TestRoom_Update(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	room := createRoom(t, app, f.TokenA, "T002", 1500, 150)

	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{
		"room_number": "T002B", "rent_price": 1600,
	})
	r.RequireOK(t)
	got := getRoom(t, app, room.ID)
	require.Equal(t, "T002B", got.RoomNumber)
	require.Equal(t, 1600.0, *got.RentPrice)

	dup := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{
		"room_number": "A101",
	})
	dup.RequireError(t, http.StatusConflict, 409)

	same := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{
		"room_number": "T002B",
	})
	same.RequireOK(t)

	cross := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["B201"].ID), f.TokenA, map[string]any{"room_number": "HACK"})
	cross.RequireError(t, http.StatusNotFound, 404)

	missing := app.Client.Do(http.MethodPut, "/api/building/rooms/999999", f.TokenA, map[string]any{"room_number": "X"})
	missing.RequireError(t, http.StatusNotFound, 404)
}

func TestRoom_UpdateEdgeCases(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	t.Run("negative rent", func(t *testing.T) {
		room := createRoom(t, app, f.TokenA, "T003", 1500, 150)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{"rent_price": -1})
		r.RequireOK(t)
		require.Equal(t, -1.0, *getRoom(t, app, room.ID).RentPrice)
	})

	t.Run("deposit months 4", func(t *testing.T) {
		room := createRoom(t, app, f.TokenA, "T004", 1500, 150)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{"deposit_months": 4})
		r.RequireOK(t)
	})
}

func TestRoom_Delete(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	room := createRoom(t, app, f.TokenA, "T005", 1500, 150)
	r := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil)
	r.RequireOK(t)
	require.Zero(t, countWhere(t, app.DB, &models.Room{}, "id = ?", room.ID))

	active := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["A102"].ID), f.TokenA, nil)
	active.RequireError(t, http.StatusBadRequest, 400).
		RequireMessageContains(t, "活跃合同")

	cross := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["B201"].ID), f.TokenA, nil)
	cross.RequireError(t, http.StatusNotFound, 404)

	// 当前删除逻辑只检查 active 合同
	t.Run("reserved contract delete", func(t *testing.T) {
		r := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["A103"].ID), f.TokenA, nil)
		r.RequireOK(t)
	})
}

func TestRoom_List(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	type roomResp struct {
		Rooms []roomRow `json:"rooms"`
		Total int       `json:"total"`
		Page  int       `json:"page"`
		Size  int       `json:"size"`
	}
	get := func(q string) roomResp {
		r := app.Client.Do(http.MethodGet, "/api/building/rooms"+q, f.TokenA, nil)
		r.RequireOK(t)
		var d roomResp
		r.Decode(t, &d)
		return d
	}

	require.ElementsMatch(t, []uint{f.Rooms["A101"].ID, f.Rooms["A106"].ID, f.Rooms["A107"].ID}, roomIDs(get("?status=vacant").Rooms))
	require.ElementsMatch(t, []uint{f.Rooms["A104"].ID}, roomIDs(get("?status=expiring").Rooms))
	require.ElementsMatch(t, []uint{f.Rooms["A102"].ID, f.Rooms["A105"].ID}, roomIDs(get("?status=rented").Rooms))
	require.ElementsMatch(t, []uint{f.Rooms["A103"].ID}, roomIDs(get("?status=reserved").Rooms))
	require.ElementsMatch(t, []uint{f.Rooms["A101"].ID}, roomIDs(get("?floor=1&layout=单间").Rooms))

	page := get("?page=1&page_size=2")
	require.Equal(t, 1, page.Page)
	require.Equal(t, 2, page.Size)
	require.Len(t, page.Rooms, 2)
	require.EqualValues(t, 7, page.Total)

	first := get("?page=1&page_size=2")
	lastID := first.Rooms[len(first.Rooms)-1].ID
	cursor := get("?last_id=" + itoa(lastID) + "&page_size=2")
	require.Equal(t, -1, cursor.Total)
	for _, room := range cursor.Rooms {
		require.Greater(t, room.ID, lastID)
	}
}

func TestRoom_Detail(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["A105"].ID), f.TokenA, nil)
	r.RequireOK(t)
	var data struct {
		Room struct {
			ID                uint                   `json:"id"`
			CurrentContract   *models.RentalContract `json:"current_contract"`
			FutureReservation *models.RentalContract `json:"future_reservation"`
			EndDate           string                 `json:"end_date"`
		} `json:"room"`
	}
	r.Decode(t, &data)
	require.Equal(t, f.Rooms["A105"].ID, data.Room.ID)
	require.NotNil(t, data.Room.CurrentContract)
	require.NotNil(t, data.Room.FutureReservation)
	require.Equal(t, "active", data.Room.CurrentContract.Status)
	require.Equal(t, "reserved", data.Room.FutureReservation.Status)

	app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["B201"].ID), f.TokenA, nil).
		RequireError(t, http.StatusNotFound, 404)
}

func TestRoom_Contracts(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodGet, "/api/building/contracts?status=active", f.TokenA, nil)
	r.RequireOK(t)
	var data struct {
		Contracts []models.RentalContract `json:"contracts"`
		Total     int                     `json:"total"`
	}
	r.Decode(t, &data)
	require.Equal(t, 3, data.Total)

	kw := app.Client.Do(http.MethodGet, "/api/building/contracts?keyword=张三", f.TokenA, nil)
	kw.RequireOK(t)
	kw.Decode(t, &data)
	require.Equal(t, 1, data.Total)

	byRoom := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/contracts?room_id=%d", f.Rooms["A102"].ID), f.TokenA, nil)
	byRoom.RequireOK(t)
	byRoom.Decode(t, &data)
	require.Equal(t, 1, data.Total)

	active := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["A102"].ID), f.TokenA, nil)
	active.RequireOK(t)

	app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["A101"].ID), f.TokenA, nil).
		RequireError(t, http.StatusNotFound, 404)
}

func TestRoom_RenewContract(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	newEnd := f.ContractA102.EndDate
	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["A102"].ID), f.TokenA, map[string]any{
		"end_date": newEnd, "rent_price": 2500,
	})
	r.RequireOK(t)
	got := getContract(t, app, f.ContractA102.ID)
	require.Equal(t, 2500.0, got.RentPrice)

	bad := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["A102"].ID), f.TokenA, map[string]any{
		"end_date": "2026/13/01",
	})
	bad.RequireError(t, http.StatusBadRequest, 400)

	noContract := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["A101"].ID), f.TokenA, map[string]any{
		"end_date": "2027-01-01",
	})
	noContract.RequireError(t, http.StatusNotFound, 404)

	cross := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["B202"].ID), f.TokenA, map[string]any{
		"end_date": "2027-01-01",
	})
	cross.RequireError(t, http.StatusNotFound, 404)
}

func TestRoom_Rent(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)

	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, rentBody(f.Rooms["A101"].ID, start, end))
	r.RequireOK(t)
	require.Equal(t, "rented", getRoom(t, app, f.Rooms["A101"].ID).Status)

	active := countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A101"].ID, "active")
	require.EqualValues(t, 1, active)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A101"].ID, "租金"))
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A101"].ID, "押金"))

	var rent models.Bill
	require.NoError(t, app.DB.Where("room_id = ? AND subtype = ?", f.Rooms["A101"].ID, "租金").First(&rent).Error)
	require.InDelta(t, 2200.0, rent.Amount, 0.01)
}

func TestRoom_RentValidation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"missing management fee", map[string]any{"status": "rented", "tenant_name": "x", "rent_price": 2000, "deposit": 2000, "start_date": start, "end_date": end}},
		{"empty dates", map[string]any{"status": "rented", "tenant_name": "x", "rent_price": 2000, "management_fee": 200, "deposit": 2000}},
		{"bad start format", map[string]any{"status": "rented", "tenant_name": "x", "rent_price": 2000, "management_fee": 200, "deposit": 2000, "start_date": "2026/10/01", "end_date": end}},
		{"end before start", map[string]any{"status": "rented", "tenant_name": "x", "rent_price": 2000, "management_fee": 200, "deposit": 2000, "start_date": end, "end_date": start}},
		{"negative rent", map[string]any{"status": "rented", "tenant_name": "x", "rent_price": -1, "management_fee": 200, "deposit": 2000, "start_date": start, "end_date": end}},
		{"negative deposit", map[string]any{"status": "rented", "tenant_name": "x", "rent_price": 2000, "management_fee": 200, "deposit": -1, "start_date": start, "end_date": end}},
		{"invalid status", map[string]any{"status": "unknown"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, tc.body)
			r.RequireError(t, http.StatusBadRequest, 400)
		})
	}

	require.Equal(t, "vacant", getRoom(t, app, f.Rooms["A101"].ID).Status)
	require.Zero(t, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ?", f.Rooms["A101"].ID))
	require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "room_id = ?", f.Rooms["A101"].ID))
}

func TestRoom_RentReplacesOldContract(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)

	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A102"].ID), f.TokenA, rentBody(f.Rooms["A102"].ID, start, end))
	r.RequireOK(t)

	old := getContract(t, app, f.ContractA102.ID)
	require.Equal(t, "ended", old.Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A102"].ID, "active"))
}

func TestRoom_Reserve(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)

	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, reserveBody(f.Rooms["A101"].ID, start, end, 500))
	r.RequireOK(t)
	require.Equal(t, "reserved", getRoom(t, app, f.Rooms["A101"].ID).Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A101"].ID, "reserved"))
	require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "room_id = ?", f.Rooms["A101"].ID))
}

func TestRoom_ReserveValidation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	start := testkit.MonthDay(f.ThisMonth, 1)
	end := testkit.MonthLastDay(f.ThisMonth)

	cases := []struct {
		name string
		body map[string]any
		room string
	}{
		{"missing tenant", map[string]any{"status": "reserved", "rent_price": 2000, "management_fee": 200, "earnest_money": 500, "start_date": start, "end_date": end}, "A101"},
		{"zero earnest", map[string]any{"status": "reserved", "tenant_name": "x", "rent_price": 2000, "management_fee": 200, "earnest_money": 0, "start_date": start, "end_date": end}, "A101"},
		{"missing mgmt", map[string]any{"status": "reserved", "tenant_name": "x", "rent_price": 2000, "earnest_money": 500, "start_date": start, "end_date": end}, "A101"},
		{"already reserved", reserveBody(0, start, end, 500), "A103"},
		{"duplicate future", reserveBody(0, f.ContractA105.EndDate, end, 500), "A105"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms[tc.room].ID), f.TokenA, tc.body)
			r.RequireError(t, http.StatusBadRequest, 400)
		})
	}
}

func TestRoom_FutureReservation(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// A104: 当前合同 D+10 结束，未来预订 D+11 起
	start := f.ContractA104.EndDate
	end := f.ContractA105Future.EndDate
	body := reserveBody(f.Rooms["A104"].ID, start, end, 500)
	body["rent_price"] = 2100
	body["management_fee"] = 200
	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A104"].ID), f.TokenA, body)
	// 起租日等于当前合同结束日：实现要求 start >= end，允许
	r.RequireOK(t)
	require.Equal(t, "rented", getRoom(t, app, f.Rooms["A104"].ID).Status)
	require.EqualValues(t, 1, countWhere(t, app.DB, &models.RentalContract{}, "room_id = ? AND status = ?", f.Rooms["A104"].ID, "reserved"))
	require.Equal(t, "active", getContract(t, app, f.ContractA104.ID).Status)

	beforeEnd := testkit.MonthDay(f.ThisMonth, 1)
	bad := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A104"].ID), f.TokenA, reserveBody(f.Rooms["A104"].ID, beforeEnd, end, 500))
	bad.RequireError(t, http.StatusBadRequest, 400)
}

func TestRoom_CancelReservation(t *testing.T) {
	cases := []struct {
		name     string
		refund   float64
		billType string
		amount   float64
	}{
		{"full forfeit", 0, "income", 500},
		{"partial refund", 200, "income", 300},
		{"over refund", 700, "expense", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, f := testkit.NewAppWithFixture(t)
			r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A103"].ID), f.TokenA, map[string]any{
				"status": "vacant", "refunded_deposit": tc.refund,
			})
			r.RequireOK(t)
			require.Equal(t, "vacant", getRoom(t, app, f.Rooms["A103"].ID).Status)
			require.Equal(t, "cancelled", getContract(t, app, f.ContractA103.ID).Status)

			var bill models.Bill
			require.NoError(t, app.DB.Where("room_id = ? AND subtype = ?", f.Rooms["A103"].ID, "定金违约").First(&bill).Error)
			require.Equal(t, tc.billType, bill.Type)
			require.InDelta(t, tc.amount, bill.Amount, 0.01)
		})
	}

	t.Run("missing refund", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A103"].ID), f.TokenA, map[string]any{"status": "vacant"})
		r.RequireError(t, http.StatusBadRequest, 400)
	})
	t.Run("negative refund", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A103"].ID), f.TokenA, map[string]any{"status": "vacant", "refunded_deposit": -1})
		r.RequireError(t, http.StatusBadRequest, 400)
	})
}

func TestRoom_Vacate(t *testing.T) {
	t.Run("refund and complete task", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A104"].ID), f.TokenA, map[string]any{
			"status": "vacant", "refunded_deposit": 1800,
		})
		r.RequireOK(t)
		require.Equal(t, "vacant", getRoom(t, app, f.Rooms["A104"].ID).Status)
		require.Equal(t, "ended", getContract(t, app, f.ContractA104.ID).Status)
		require.Equal(t, "completed", getTask(t, app, f.TaskA104.ID).Status)
		require.EqualValues(t, 1, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A104"].ID, "押金退还"))
	})

	t.Run("zero refund no bill", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A102"].ID), f.TokenA, map[string]any{"status": "vacant", "refunded_deposit": 0})
		r.RequireOK(t)
		require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", f.Rooms["A102"].ID, "押金退还"))
	})

	t.Run("no active contract", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A101"].ID), f.TokenA, map[string]any{"status": "vacant", "refunded_deposit": 100})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("missing refund", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A102"].ID), f.TokenA, map[string]any{"status": "vacant"})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("negative refund", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A102"].ID), f.TokenA, map[string]any{"status": "vacant", "refunded_deposit": -1})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("future reservation keeps reserved", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["A105"].ID), f.TokenA, map[string]any{"status": "vacant", "refunded_deposit": 2200})
		r.RequireOK(t)
		require.Equal(t, "reserved", getRoom(t, app, f.Rooms["A105"].ID).Status)
	})
}

func TestRoom_BatchPricing(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{
		"room_ids":   []uint{f.Rooms["A101"].ID, f.Rooms["A106"].ID},
		"rent_delta": 100.0, "management_fee": 250.0, "deposit_months": 2,
	})
	r.RequireOK(t)
	require.Equal(t, 2100.0, *getRoom(t, app, f.Rooms["A101"].ID).RentPrice)
	require.Equal(t, 250.0, *getRoom(t, app, f.Rooms["A106"].ID).ManagementFee)
	require.EqualValues(t, 2, *getRoom(t, app, f.Rooms["A101"].ID).DepositMonths)

	cross := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{
		"room_ids": []uint{f.Rooms["A101"].ID, f.Rooms["B201"].ID}, "rent_delta": 1.0,
	})
	cross.RequireError(t, http.StatusForbidden, 403)

	empty := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{"room_ids": []uint{}})
	empty.RequireError(t, http.StatusBadRequest, 400)

	noField := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{"room_ids": []uint{f.Rooms["A101"].ID}})
	noField.RequireError(t, http.StatusBadRequest, 400)

	deposit := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{"room_ids": []uint{f.Rooms["A101"].ID}, "deposit_months": 4})
	deposit.RequireError(t, http.StatusBadRequest, 400)

	negative := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{"room_ids": []uint{f.Rooms["A101"].ID}, "management_fee": -1.0})
	negative.RequireError(t, http.StatusBadRequest, 400)

	// 租金减到负数时钳制为 0
	require.NoError(t, app.DB.Model(&models.Room{}).Where("id = ?", f.Rooms["A101"].ID).Update("rent_price", 100.0).Error)
	clamp := app.Client.Do(http.MethodPost, "/api/building/rooms/batch-pricing", f.TokenA, map[string]any{"room_ids": []uint{f.Rooms["A101"].ID}, "rent_delta": -1000.0})
	clamp.RequireOK(t)
	require.Equal(t, 0.0, *getRoom(t, app, f.Rooms["A101"].ID).RentPrice)
}

func TestRoom_ImportRented(t *testing.T) {
	baseItem := func(number string) map[string]any {
		return map[string]any{
			"room_number": number, "floor": "8", "layout": "单间",
			"rent_price": 1800, "deposit_months": 1, "management_fee": 180,
			"electricity_unit_price": 1.2, "water_unit_price": 4,
			"tenant_name": "导入租客", "tenant_phone": "13933334444",
			"deposit": 1800, "start_date": testkit.MonthDay("2026-01", 1), "end_date": "2027-01-01",
		}
	}

	t.Run("success", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		item := baseItem("I001")
		item["start_date"] = f.Today
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": []any{item}})
		r.RequireOK(t)
		var data struct {
			SuccessCount int `json:"success_count"`
			Failures     []struct {
				RoomNumber string `json:"room_number"`
				Reason     string `json:"reason"`
			} `json:"failures"`
		}
		r.Decode(t, &data)
		require.Equal(t, 1, data.SuccessCount)
		require.Empty(t, data.Failures)

		var room models.Room
		require.NoError(t, app.DB.Where("room_number = ?", "I001").First(&room).Error)
		require.Equal(t, "rented", room.Status)
		require.Zero(t, countWhere(t, app.DB, &models.Bill{}, "room_id = ? AND subtype = ?", room.ID, "押金"))
	})

	t.Run("duplicate room", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		item := baseItem("A101")
		item["start_date"] = f.Today
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": []any{item}})
		r.RequireOK(t)
		var data struct {
			SuccessCount int `json:"success_count"`
			Failures     []struct {
				Reason string `json:"reason"`
			} `json:"failures"`
		}
		r.Decode(t, &data)
		require.Equal(t, 0, data.SuccessCount)
		require.Len(t, data.Failures, 1)
		require.Contains(t, data.Failures[0].Reason, "导入失败")
	})

	t.Run("past end date", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		item := baseItem("I002")
		item["start_date"] = "2020-01-01"
		item["end_date"] = "2020-12-31"
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": []any{item}})
		r.RequireOK(t)
		var data struct {
			SuccessCount int `json:"success_count"`
			Failures     []struct {
				Reason string `json:"reason"`
			} `json:"failures"`
		}
		r.Decode(t, &data)
		require.Equal(t, 0, data.SuccessCount)
		require.Contains(t, data.Failures[0].Reason, "不能早于今天")
	})

	t.Run("too many rooms", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		rooms := make([]any, 201)
		for i := range rooms {
			item := baseItem(fmt.Sprintf("M%03d", i))
			item["start_date"] = f.Today
			rooms[i] = item
		}
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": rooms})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("empty rooms", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": []any{}})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("mixed batch", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		ok := baseItem("I003")
		ok["start_date"] = f.Today
		dup := baseItem("A101")
		dup["start_date"] = f.Today
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": []any{ok, dup}})
		r.RequireOK(t)
		var data struct {
			SuccessCount int `json:"success_count"`
			Failures     []struct {
				RoomNumber string `json:"room_number"`
			} `json:"failures"`
		}
		r.Decode(t, &data)
		require.Equal(t, 1, data.SuccessCount)
		require.Len(t, data.Failures, 1)
		require.Equal(t, "A101", data.Failures[0].RoomNumber)
	})

	t.Run("tenant reuse", func(t *testing.T) {
		app, f := testkit.NewAppWithFixture(t)
		a := baseItem("I004")
		a["start_date"] = f.Today
		b := baseItem("I005")
		b["start_date"] = f.Today
		r := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{"rooms": []any{a, b}})
		r.RequireOK(t)
		var tenant models.Tenant
		require.NoError(t, app.DB.Where("name = ? AND phone = ?", "导入租客", "13933334444").First(&tenant).Error)
		var contracts []models.RentalContract
		require.NoError(t, app.DB.Where("tenant_id = ?", tenant.ID).Find(&contracts).Error)
		require.Len(t, contracts, 2)
	})
}
