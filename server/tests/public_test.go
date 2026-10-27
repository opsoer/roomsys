package tests

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

type buildingRow struct {
	ID        uint    `json:"id"`
	Name      string  `json:"name"`
	Deposit   float64 `json:"deposit"`
	RoomCount int64   `json:"room_count"`
	Landlords []struct {
		Phone string `json:"phone"`
	} `json:"landlords"`
}

type buildingListResp struct {
	Buildings []buildingRow `json:"buildings"`
	Total     int           `json:"total"`
	Page      int           `json:"page"`
	Size      int           `json:"size"`
}

type roomRow struct {
	ID      uint   `json:"id"`
	Status  string `json:"status"`
	EndDate string `json:"end_date"`
}

type roomListResp struct {
	Rooms []roomRow `json:"rooms"`
	Total int       `json:"total"`
}

func itoa(u uint) string { return strconv.FormatUint(uint64(u), 10) }

func roomIDs(rows []roomRow) []uint {
	out := make([]uint, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out
}

func buildingIDs(rows []buildingRow) []uint {
	out := make([]uint, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out
}

func TestPublic_BuildingList(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	r := app.Client.Do(http.MethodGet, "/api/buildings", "", nil)
	r.RequireOK(t)
	var data buildingListResp
	r.Decode(t, &data)
	require.Equal(t, 4, data.Total)
	require.Len(t, data.Buildings, 4)
	for _, b := range data.Buildings {
		require.Zero(t, b.Deposit, "公开端不应返回押金")
		for _, l := range b.Landlords {
			require.Contains(t, l.Phone, "****")
			require.NotRegexp(t, `^\d{11}$`, l.Phone)
		}
	}
}

func TestPublic_HiddenAndExpiredExcluded(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.D.ID).
		Update("status", "hidden").Error)

	list := app.Client.Do(http.MethodGet, "/api/buildings", "", nil)
	list.RequireOK(t)
	var data buildingListResp
	list.Decode(t, &data)
	require.Equal(t, 3, data.Total)
	for _, b := range data.Buildings {
		require.NotEqual(t, f.D.ID, b.ID)
	}
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.D.ID), "", nil).
		RequireError(t, http.StatusNotFound, 404)
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.D.ID)+"/rooms", "", nil).
		RequireError(t, http.StatusNotFound, 404)

	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.D.ID).
		Updates(map[string]any{"status": "active", "expired_at": "2020-01-01"}).Error)
	list2 := app.Client.Do(http.MethodGet, "/api/buildings", "", nil)
	list2.RequireOK(t)
	list2.Decode(t, &data)
	require.Equal(t, 3, data.Total)
}

func TestPublic_Filters(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	ids := func(q string) []uint {
		r := app.Client.Do(http.MethodGet, "/api/buildings"+q, "", nil)
		r.RequireOK(t)
		var d buildingListResp
		r.Decode(t, &d)
		return buildingIDs(d.Buildings)
	}

	require.ElementsMatch(t, []uint{f.A.ID, f.B.ID}, ids("?district=南山区"))
	require.ElementsMatch(t, []uint{f.A.ID, f.B.ID}, ids("?street=科技园街道"))
	require.ElementsMatch(t, []uint{f.A.ID, f.B.ID}, ids("?village=测试花园"))
	require.ElementsMatch(t, []uint{f.A.ID}, ids("?min_price=2000&max_price=2200"))
	require.Empty(t, ids("?min_price=99999"))
	require.ElementsMatch(t, []uint{f.A.ID}, ids("?layout=两房"))
	require.ElementsMatch(t, []uint{f.A.ID}, ids("?district=南山区&street=科技园&village=测试花园&min_price=2000&max_price=2200&layout=单间"))
}

func TestPublic_Pagination(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	r := app.Client.Do(http.MethodGet, "/api/buildings?page=2&page_size=2", "", nil)
	r.RequireOK(t)
	var data buildingListResp
	r.Decode(t, &data)
	require.Equal(t, 4, data.Total)
	require.Equal(t, 2, data.Page)
	require.Equal(t, 2, data.Size)
	require.Len(t, data.Buildings, 2)

	first := app.Client.Do(http.MethodGet, "/api/buildings?page=1&page_size=2", "", nil)
	first.RequireOK(t)
	var firstData buildingListResp
	first.Decode(t, &firstData)
	lastID := firstData.Buildings[len(firstData.Buildings)-1].ID

	cursor := app.Client.Do(http.MethodGet, "/api/buildings?last_id="+itoa(lastID)+"&page_size=2", "", nil)
	cursor.RequireOK(t)
	var cursorData buildingListResp
	cursor.Decode(t, &cursorData)
	require.Equal(t, -1, cursorData.Total)
	for _, b := range cursorData.Buildings {
		require.Greater(t, b.ID, lastID)
	}
}

func TestPublic_BuildingDetail(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	r := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID), "", nil)
	r.RequireOK(t)
	var data struct {
		Building buildingRow `json:"building"`
	}
	r.Decode(t, &data)
	require.Equal(t, f.A.ID, data.Building.ID)
	require.Zero(t, data.Building.Deposit)
	require.NotEmpty(t, data.Building.Landlords)
	for _, l := range data.Building.Landlords {
		require.Contains(t, l.Phone, "****")
	}

	app.Client.Do(http.MethodGet, "/api/buildings/abc", "", nil).
		RequireError(t, http.StatusBadRequest, 400)
}

func TestPublic_RoomsVisibility(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	get := func(q string) roomListResp {
		r := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID)+"/rooms"+q, "", nil)
		r.RequireOK(t)
		var d roomListResp
		r.Decode(t, &d)
		return d
	}

	all := get("")
	for _, room := range all.Rooms {
		require.NotEqual(t, f.Rooms["A103"].ID, room.ID, "预订中房间不应出现在公开列表")
	}

	require.ElementsMatch(t, []uint{
		f.Rooms["A101"].ID, f.Rooms["A106"].ID, f.Rooms["A107"].ID,
	}, roomIDs(get("?status=vacant").Rooms))
	require.ElementsMatch(t, []uint{f.Rooms["A104"].ID}, roomIDs(get("?status=expiring").Rooms))
	// 公开端基础过滤只允许 vacant 或 30 天内到期的 rented，因此非管理员请求
	// status=rented 与基础过滤交集为空（当前实现行为）。
	require.Empty(t, roomIDs(get("?status=rented").Rooms))
	require.ElementsMatch(t, []uint{f.Rooms["A101"].ID}, roomIDs(get("?floor=1&layout=单间").Rooms))

	// 本栋管理员可见全部房间
	admin := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID)+"/rooms?status=rented", f.TokenA, nil)
	admin.RequireOK(t)
	var adminData roomListResp
	admin.Decode(t, &adminData)
	// A105 因存在未来 reserved 合同，被基础查询的 NOT IN reserved 排除（当前实现行为）
	require.ElementsMatch(t, []uint{f.Rooms["A102"].ID}, roomIDs(adminData.Rooms))
}

func TestPublic_RoomDetailAndContractLeak(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	t.Run("public no contract", func(t *testing.T) {
		r := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID)+"/rooms/"+itoa(f.Rooms["A102"].ID), "", nil)
		r.RequireOK(t)
		require.NotContains(t, string(r.Raw), "current_contract")
		require.NotContains(t, string(r.Raw), "13900000001")
	})

	t.Run("same building auth sees contract", func(t *testing.T) {
		r := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID)+"/rooms/"+itoa(f.Rooms["A102"].ID), f.TokenA, nil)
		r.RequireOK(t)
		require.Contains(t, string(r.Raw), "current_contract")
	})

	t.Run("cross building 404", func(t *testing.T) {
		r := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.B.ID)+"/rooms/"+itoa(f.Rooms["A102"].ID), "", nil)
		r.RequireError(t, http.StatusNotFound, 404)
	})
}

func TestPublic_LocationsAndOptions(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	r := app.Client.Do(http.MethodGet, "/api/buildings/locations", "", nil)
	r.RequireOK(t)
	require.Contains(t, string(r.Raw), "南山区")
	require.Contains(t, string(r.Raw), "科技园")

	opts := app.Client.Do(http.MethodGet, "/api/locations/options", "", nil)
	opts.RequireOK(t)
	require.NotContains(t, string(opts.Raw), "\"id\"")
	require.NotContains(t, string(opts.Raw), "created_at")
}

func TestPublic_RevealPhoneQuota(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	for i := 0; i < 5; i++ {
		r := app.Client.Do(http.MethodPost, "/api/buildings/"+itoa(f.A.ID)+"/reveal-phone", "", map[string]any{})
		r.RequireOK(t)
		require.Contains(t, string(r.Raw), "13800000001")
	}
	sixth := app.Client.Do(http.MethodPost, "/api/buildings/"+itoa(f.A.ID)+"/reveal-phone", "", map[string]any{})
	sixth.RequireError(t, http.StatusTooManyRequests, 1007)

	bad := app.Client.Do(http.MethodPost, "/api/buildings/"+itoa(f.A.ID)+"/reveal-phone", "", map[string]any{
		"captcha_id": "bad", "captcha_code": "0000",
	})
	bad.RequireError(t, http.StatusTooManyRequests, 1007)
}

func TestPublic_CaptchaAndConfig(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	c := app.Client.Do(http.MethodGet, "/api/captcha", "", nil)
	c.RequireOK(t)
	var data struct {
		CaptchaID string `json:"captcha_id"`
		Image     string `json:"image"`
	}
	c.Decode(t, &data)
	require.NotEmpty(t, data.CaptchaID)
	require.True(t, strings.HasPrefix(data.Image, "data:image"))

	cfg := app.Client.Do(http.MethodGet, "/api/config", "", nil)
	cfg.RequireOK(t)
	require.NotContains(t, string(cfg.Raw), "access_key")
	require.NotContains(t, string(cfg.Raw), "secret_key")
}

func TestPublic_RecruitSubmit(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	r := app.Client.Do(http.MethodPost, "/api/recruit/submit", "", map[string]any{
		"phone": "13877778888", "address": "测试区测试街道1号",
	})
	r.RequireOK(t)

	var sub models.RecruitSubmission
	require.NoError(t, app.DB.Where("phone = ?", "13877778888").First(&sub).Error)
	require.Equal(t, "pending", sub.Status)

	var task models.Task
	require.NoError(t, app.DB.Where("type = ? AND scope = ?", "recruit", "platform").First(&task).Error)
	require.Equal(t, sub.ID, *task.RefID)

	bad := app.Client.Do(http.MethodPost, "/api/recruit/submit", "", map[string]any{"phone": "1"})
	bad.RequireError(t, http.StatusBadRequest, 400)
}
