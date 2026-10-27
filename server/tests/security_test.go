package tests

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
	"rental-server/utils"
)

func TestSecurity_IDORMatrix(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	bShare := models.Shareholder{BuildingID: f.B.ID, Name: "B股东", ShareRatio: 100}
	require.NoError(t, app.DB.Create(&bShare).Error)
	bTask := models.Task{BuildingID: &f.B.ID, Title: "B待办", Type: "expired_room", Status: "pending", RoomID: uptr(f.Rooms["B202"].ID), Scope: "building"}
	require.NoError(t, app.DB.Create(&bTask).Error)
	bMedia := models.RoomMedia{RoomID: f.Rooms["B201"].ID, Type: "image", Category: "gallery", FilePath: "buildings/b/x.png", Status: "ready"}
	require.NoError(t, app.DB.Create(&bMedia).Error)
	var bBill models.Bill
	require.NoError(t, app.DB.Where("building_id = ?", f.B.ID).First(&bBill).Error)

	cases := []struct {
		name   string
		method string
		path   string
		body   any
		status int
		code   int
	}{
		{"read B room", http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["B201"].ID), nil, 404, 404},
		{"update B room", http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["B201"].ID), map[string]any{"room_number": "HACK"}, 404, 404},
		{"delete B room", http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["B201"].ID), nil, 404, 404},
		{"rent B room", http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["B201"].ID), rentBody(f.Rooms["B201"].ID, f.Today, f.Today), 404, 404},
		{"read B contract", http.MethodGet, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["B202"].ID), nil, 404, 404},
		{"update B contract", http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/contract", f.Rooms["B202"].ID), map[string]any{"end_date": "2027-01-01"}, 404, 404},
		{"vacate B room", http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", f.Rooms["B202"].ID), map[string]any{"status": "vacant", "refunded_deposit": 0}, 404, 404},
		{"delete B media", http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d/media/%d", f.Rooms["B201"].ID, bMedia.ID), nil, 403, 403},
		{"update B bill", http.MethodPut, fmt.Sprintf("/api/building/bills/%d", bBill.ID), map[string]any{"amount": 1.0, "modify_reason": "x"}, 404, 404},
		{"delete B bill", http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", bBill.ID), nil, 404, 404},
		{"update B shareholder", http.MethodPut, fmt.Sprintf("/api/building/dividends/shareholders/%d", bShare.ID), map[string]any{"share_ratio": 1.0}, 404, 404},
		{"delete B shareholder", http.MethodDelete, fmt.Sprintf("/api/building/dividends/shareholders/%d", bShare.ID), nil, 404, 404},
		{"process B task", http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", bTask.ID), map[string]any{"refunded_deposit": 0}, 404, 404},
		{"complete B task", http.MethodPut, fmt.Sprintf("/api/building/tasks/%d/complete", bTask.ID), nil, 404, 404},
		{"delete B task", http.MethodDelete, fmt.Sprintf("/api/building/tasks/%d", bTask.ID), nil, 404, 404},
		{"bill to B room", http.MethodPost, "/api/building/bills", map[string]any{"type": "income", "subtype": "租金", "amount": 1, "bill_date": f.Today, "room_id": f.Rooms["B201"].ID}, 400, 400},
		{"batch includes B", http.MethodPost, "/api/building/rooms/batch-pricing", map[string]any{"room_ids": []uint{f.Rooms["A101"].ID, f.Rooms["B201"].ID}, "rent_delta": 1.0}, 403, 403},
		{"create copies B room", http.MethodPost, "/api/building/rooms", map[string]any{"room_number": "SEC1", "floor": "1", "layout": "单间", "rent_price": 1, "deposit_months": 1, "management_fee": 1, "electricity_unit_price": 1, "water_unit_price": 1, "copy_from_room_id": f.Rooms["B201"].ID}, 400, 400},
		{"copy B media source", http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media/copy", f.Rooms["A106"].ID), map[string]any{"source_room_number": "B201"}, 404, 404},
		{"upload to B room", http.MethodPost, fmt.Sprintf("/api/building/rooms/%d/media", f.Rooms["B201"].ID), nil, 404, 404},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var r *testkit.Response
			if tc.method == http.MethodPost && tc.path == fmt.Sprintf("/api/building/rooms/%d/media", f.Rooms["B201"].ID) {
				r = uploadMedia(t, app, f.TokenA, f.Rooms["B201"].ID, "x.png", pngBytes, "gallery")
			} else {
				r = app.Client.Do(tc.method, tc.path, f.TokenA, tc.body)
			}
			r.RequireStatus(t, tc.status, tc.code)
		})
	}

	// B 栋数据未被修改
	require.Equal(t, "B201", getRoom(t, app, f.Rooms["B201"].ID).RoomNumber)
	require.Equal(t, "active", getContract(t, app, f.ContractB202.ID).Status)
	require.Equal(t, 100.0, func() float64 {
		var s models.Shareholder
		require.NoError(t, app.DB.First(&s, bShare.ID).Error)
		return s.ShareRatio
	}())
}

func TestSecurity_JWT(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	t.Run("tampered", func(t *testing.T) {
		token := f.TokenA[:len(f.TokenA)-2] + "xx"
		app.Client.Do(http.MethodGet, "/api/building/info", token, nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("wrong secret", func(t *testing.T) {
		token, err := utils.GenerateToken(f.AUser.ID, f.AUser.Username, f.AUser.Role, "wrong-secret", f.A.ID)
		require.NoError(t, err)
		app.Client.Do(http.MethodGet, "/api/building/info", token, nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("alg none", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"user_id":%d,"role":"super_admin"}`, f.Root.ID)))
		token := header + "." + payload + "."
		app.Client.Do(http.MethodGet, "/api/admin/buildings", token, nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("disabled user", func(t *testing.T) {
		token := app.TokenFor(f.Disabled)
		app.Client.Do(http.MethodGet, "/api/building/rooms", token, nil).
			RequireError(t, http.StatusUnauthorized, 401).
			RequireMessageContains(t, "账号已停用")
	})

	t.Run("deleted user", func(t *testing.T) {
		u := models.User{Username: "jwt_deleted", PasswordHash: testkit.HashFor("Test12345"), Role: "admin", BuildingID: &f.A.ID, IsActive: true}
		require.NoError(t, app.DB.Create(&u).Error)
		token := app.TokenFor(u)
		require.NoError(t, app.DB.Delete(&u).Error)
		app.Client.Do(http.MethodGet, "/api/building/info", token, nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("revoked", func(t *testing.T) {
		u := models.User{Username: "jwt_revoked", PasswordHash: testkit.HashFor("Test12345"), Role: "admin", BuildingID: &f.A.ID, IsActive: true}
		require.NoError(t, app.DB.Create(&u).Error)
		token := app.TokenFor(u)
		utils.RevokeToken(token)
		app.Client.Do(http.MethodGet, "/api/building/info", token, nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("no token", func(t *testing.T) {
		app.Client.Do(http.MethodGet, "/api/building/info", "", nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})
}

func TestSecurity_Injection(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	inject := "' OR 1=1 --"
	lr := login(t, app.Client, inject, "x")
	lr.RequireError(t, http.StatusUnauthorized, 401)

	// 参数化查询：注入串不会返回全量
	sub := app.Client.Do(http.MethodGet, "/api/building/bills?subtype="+urlEncode(inject), f.TokenA, nil)
	sub.RequireOK(t)
	var bills struct {
		Total int `json:"total"`
	}
	sub.Decode(t, &bills)
	require.Zero(t, bills.Total)

	pub := app.Client.Do(http.MethodGet, "/api/buildings?district="+urlEncode(inject), "", nil)
	pub.RequireOK(t)
	var buildings struct {
		Total int `json:"total"`
	}
	pub.Decode(t, &buildings)
	require.Zero(t, buildings.Total)

	// 非法月份显式 400（BUG-010 修复后），且不泄露 SQL
	bad := app.Client.Do(http.MethodGet, "/api/building/bills/stats?month=2026-13", f.TokenA, nil)
	bad.RequireStatus(t, http.StatusBadRequest, 400)
	require.NotContains(t, string(bad.Raw), "SQL")
	require.NotContains(t, string(bad.Raw), "sqlite")
	require.NotContains(t, string(bad.Raw), "bills")
}

func TestSecurity_RateLimit(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	c := testkit.NewClient(t, app.Server.URL, "10.250.250.252")
	limit := 240
	if v := os.Getenv("RATE_LIMIT_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	for i := 0; i < limit; i++ {
		r := c.Do(http.MethodGet, "/api/config", "", nil)
		require.Equal(t, http.StatusOK, r.Status, "request %d", i+1)
	}
	blocked := c.Do(http.MethodGet, "/api/config", "", nil)
	blocked.RequireError(t, http.StatusTooManyRequests, 429)
}

func TestSecurity_SensitiveFields(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	lr := login(t, app.Client, "root", "root")
	lr.RequireOK(t)
	require.NotContains(t, string(lr.Raw), "password_hash")

	users := app.Client.Do(http.MethodGet, "/api/admin/auth/users", f.TokenRoot, nil)
	users.RequireOK(t)
	require.NotContains(t, string(users.Raw), "password_hash")
	require.NotContains(t, string(users.Raw), "id_card")

	pub := app.Client.Do(http.MethodGet, "/api/buildings", "", nil)
	pub.RequireOK(t)
	require.NotContains(t, string(pub.Raw), "13800000001")
	require.NotContains(t, string(pub.Raw), "\"deposit\":5000")

	room := app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID)+"/rooms/"+itoa(f.Rooms["A102"].ID), "", nil)
	room.RequireOK(t)
	require.NotContains(t, string(room.Raw), "id_card")
	require.NotContains(t, string(room.Raw), "13900000001")

	cfg := app.Client.Do(http.MethodGet, "/api/config", "", nil)
	cfg.RequireOK(t)
	require.NotContains(t, string(cfg.Raw), "access_key")
	require.NotContains(t, string(cfg.Raw), "secret_key")
	require.NotContains(t, string(cfg.Raw), "jwt_secret")

	audit := app.Client.Do(http.MethodGet, "/api/admin/system/audit-logs", f.TokenRoot, nil)
	audit.RequireOK(t)
	require.NotContains(t, string(audit.Raw), "password")
}

func TestSecurity_SoftDelete(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	t.Run("room", func(t *testing.T) {
		room := createRoom(t, app, f.TokenA, "SD1", 1000, 100)
		app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil).RequireOK(t)
		app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil).
			RequireError(t, http.StatusNotFound, 404)
		app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{"room_number": "SD1B"}).
			RequireError(t, http.StatusNotFound, 404)
	})

	t.Run("bill", func(t *testing.T) {
		b := createBill(t, app, f.TokenA, "income", "租金", 100, f.Today, "软删除测试", nil)
		app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/bills/%d", b.ID), f.TokenA, nil).RequireOK(t)
		app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/bills/%d", b.ID), f.TokenA, map[string]any{"amount": 1.0, "modify_reason": "x"}).
			RequireError(t, http.StatusNotFound, 404)
	})

	t.Run("shareholder", func(t *testing.T) {
		sh := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{"name": "软删股东", "share_ratio": 1})
		sh.RequireOK(t)
		var sd struct {
			Shareholder models.Shareholder `json:"shareholder"`
		}
		sh.Decode(t, &sd)
		app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/dividends/shareholders/%d", sd.Shareholder.ID), f.TokenA, nil).RequireOK(t)
		app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/dividends/shareholders/%d", sd.Shareholder.ID), f.TokenA, map[string]any{"share_ratio": 2.0}).
			RequireError(t, http.StatusNotFound, 404)
	})

	t.Run("user", func(t *testing.T) {
		u := models.User{Username: "sd_user", PasswordHash: testkit.HashFor("Test12345"), Role: "admin", BuildingID: &f.A.ID, IsActive: true}
		require.NoError(t, app.DB.Create(&u).Error)
		token := app.TokenFor(u)
		app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/auth/users/%d", u.ID), f.TokenRoot, nil).RequireOK(t)
		app.Client.Do(http.MethodGet, "/api/building/info", token, nil).
			RequireError(t, http.StatusUnauthorized, 401)
	})
}

func TestSecurity_UserRoleUpdate(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	u := models.User{Username: "elevate", PasswordHash: testkit.HashFor("Test12345"), Role: "admin", BuildingID: &f.A.ID, IsActive: true}
	require.NoError(t, app.DB.Create(&u).Error)

	// 验证 root 更新用户角色后，新角色 token 的访问结果
	up := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/auth/users/%d", u.ID), f.TokenRoot, map[string]any{"role": "super_admin"})
	up.RequireOK(t)

	var got models.User
	require.NoError(t, app.DB.First(&got, u.ID).Error)
	require.Equal(t, "super_admin", got.Role)
	token := app.TokenFor(got)
	app.Client.Do(http.MethodGet, "/api/admin/buildings", token, nil).RequireOK(t)
}

func urlEncode(s string) string {
	return url.QueryEscape(s)
}
