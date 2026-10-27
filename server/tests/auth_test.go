package tests

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

func login(t *testing.T, c *testkit.Client, username, password string) *testkit.Response {
	t.Helper()
	return c.Do(http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": username, "password": password,
	})
}

func TestAuth_Login(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	t.Run("root success", func(t *testing.T) {
		r := login(t, app.Client, "root", "root")
		r.RequireOK(t)
		var data struct {
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
			User         struct {
				Role       string `json:"role"`
				BuildingID uint   `json:"building_id"`
			} `json:"user"`
		}
		r.Decode(t, &data)
		require.NotEmpty(t, data.Token)
		require.NotEmpty(t, data.RefreshToken)
		require.Equal(t, "super_admin", data.User.Role)
		require.Zero(t, data.User.BuildingID)
	})

	t.Run("wrong password", func(t *testing.T) {
		r := login(t, app.Client, "root", "wrong")
		r.RequireError(t, http.StatusUnauthorized, 401).
			RequireMessageContains(t, "用户名或密码错误")
	})

	t.Run("unknown user", func(t *testing.T) {
		r := login(t, app.Client, "not_exist_user", "x")
		r.RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("missing username", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/login", "", map[string]any{"password": "root"})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("missing password", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/login", "", map[string]any{"username": "root"})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("empty body", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/login", "", map[string]any{})
		r.RequireError(t, http.StatusBadRequest, 400)
	})
}

func TestAuth_DisabledUser(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	// 当前实现：登录接口不校验 is_active，登录本身可能成功；受保护接口由中间件拦截。
	r := login(t, app.Client, "disabled", "Test12345")
	if r.Status == http.StatusOK {
		var data struct {
			Token string `json:"token"`
		}
		r.Decode(t, &data)
		blocked := app.Client.Do(http.MethodGet, "/api/building/rooms", data.Token, nil)
		blocked.RequireError(t, http.StatusUnauthorized, 401).
			RequireMessageContains(t, "账号已停用")
	} else {
		r.RequireError(t, http.StatusForbidden, 403)
	}
	_ = f
}

func TestAuth_ExpiredBuildingLogin(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	require.NoError(t, app.DB.Model(&models.Building{}).Where("id = ?", f.D.ID).
		Update("expired_at", yesterday).Error)

	r := login(t, app.Client, "dadmin", "Test12345")
	r.RequireError(t, http.StatusForbidden, 403).
		RequireMessageContains(t, "公寓已到期")
}

func TestAuth_Refresh(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	loginR := login(t, app.Client, "root", "root")
	loginR.RequireOK(t)
	var loginData struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	loginR.Decode(t, &loginData)

	t.Run("success", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/refresh", "", map[string]any{
			"refresh_token": loginData.RefreshToken,
		})
		r.RequireOK(t)
		var data struct {
			Token string `json:"token"`
		}
		r.Decode(t, &data)
		require.NotEmpty(t, data.Token)
	})

	t.Run("access token as refresh", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/refresh", "", map[string]any{
			"refresh_token": loginData.Token,
		})
		r.RequireError(t, http.StatusUnauthorized, 401).
			RequireMessageContains(t, "无效的刷新令牌")
	})

	t.Run("invalid refresh", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/refresh", "", map[string]any{
			"refresh_token": "abc.def.ghi",
		})
		r.RequireError(t, http.StatusUnauthorized, 401)
	})

	t.Run("missing refresh", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/auth/refresh", "", map[string]any{})
		r.RequireError(t, http.StatusBadRequest, 400)
	})
}

func TestAuth_Logout(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	// 用独立用户，避免与 fixture token 在同一秒生成导致 JWT 完全相同而被一起吊销
	u := models.User{Username: "logout_user", PasswordHash: testkit.HashFor("Test12345"), Role: "admin", IsActive: true, BuildingID: &f.A.ID}
	require.NoError(t, app.DB.Create(&u).Error)
	token := app.TokenFor(u)

	out := app.Client.Do(http.MethodPost, "/api/auth/logout", token, nil)
	out.RequireOK(t)

	after := app.Client.Do(http.MethodGet, "/api/building/rooms", token, nil)
	after.RequireError(t, http.StatusUnauthorized, 401)

	noHeader := app.Client.Do(http.MethodPost, "/api/auth/logout", "", nil)
	noHeader.RequireError(t, http.StatusBadRequest, 400)
	_ = f
}

func TestAuth_NoTokenAndMalformed(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	noToken := app.Client.Do(http.MethodGet, "/api/building/rooms", "", nil)
	noToken.RequireError(t, http.StatusUnauthorized, 401)

	malformed := app.Client.DoWithHeaders(http.MethodGet, "/api/building/info", "", nil,
		map[string]string{"Authorization": "Token abc"})
	malformed.RequireError(t, http.StatusUnauthorized, 401)

	tampered := "eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoxfQ.bad"
	bad := app.Client.Do(http.MethodGet, "/api/building/info", tampered, nil)
	bad.RequireError(t, http.StatusUnauthorized, 401)
}

func TestAuth_RoleMatrix(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	cases := []struct {
		name   string
		method string
		path   string
		token  string
		status int
		code   int
	}{
		{"super platform", http.MethodGet, "/api/admin/buildings", f.TokenRoot, 200, 0},
		{"building admin platform", http.MethodGet, "/api/admin/buildings", f.TokenA, 403, 403},
		{"plain admin platform", http.MethodGet, "/api/admin/auth/users", f.TokenAPlain, 403, 403},
		{"building admin own info", http.MethodGet, "/api/building/info", f.TokenA, 200, 0},
		{"plain admin own info", http.MethodGet, "/api/building/info", f.TokenAPlain, 200, 0},
		{"super landlord info", http.MethodGet, "/api/building/info", f.TokenRoot, 401, 401},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := app.Client.Do(tc.method, tc.path, tc.token, nil)
			r.RequireStatus(t, tc.status, tc.code)
		})
	}
}

func TestAuth_PackageGate(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	t.Run("basic bills 403", func(t *testing.T) {
		app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenC, nil).
			RequireError(t, http.StatusForbidden, 403).
			RequireMessageContains(t, "套餐")
	})
	t.Run("basic dividends 403", func(t *testing.T) {
		app.Client.Do(http.MethodGet, "/api/building/dividends", f.TokenC, nil).
			RequireError(t, http.StatusForbidden, 403)
	})
	t.Run("basic tasks 200", func(t *testing.T) {
		app.Client.Do(http.MethodGet, "/api/building/tasks", f.TokenC, nil).RequireOK(t)
	})
	t.Run("full bills 200", func(t *testing.T) {
		app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenA, nil).RequireOK(t)
	})
}

func TestAuth_CreateAdmin(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	t.Run("create admin", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/admin/auth/create-admin", f.TokenRoot, map[string]any{
			"username": "new_admin_01", "password": "Test12345",
		})
		r.RequireOK(t)
		var data struct {
			User struct {
				Role string `json:"role"`
			} `json:"user"`
		}
		r.Decode(t, &data)
		require.Equal(t, "building_admin", data.User.Role)
	})

	t.Run("duplicate admin", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/admin/auth/create-admin", f.TokenRoot, map[string]any{
			"username": "aadmin", "password": "Test12345",
		})
		r.RequireError(t, http.StatusConflict, 409)
	})

	t.Run("building admin nonexistent building", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/admin/auth/create-building-admin", f.TokenRoot, map[string]any{
			"username": "x", "password": "Test12345", "building_id": 999999,
		})
		r.RequireError(t, http.StatusNotFound, 404)
	})

	t.Run("building admin missing building", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/admin/auth/create-building-admin", f.TokenRoot, map[string]any{
			"username": "x", "password": "Test12345",
		})
		r.RequireError(t, http.StatusBadRequest, 400)
	})

	t.Run("regular admin", func(t *testing.T) {
		r := app.Client.Do(http.MethodPost, "/api/building/auth/create-admin", f.TokenA, map[string]any{
			"username": "a_plain_new", "password": "Test12345",
		})
		r.RequireOK(t)
		var u models.User
		require.NoError(t, app.DB.Where("username = ?", "a_plain_new").First(&u).Error)
		require.Equal(t, "admin", u.Role)
		require.NotNil(t, u.BuildingID)
		require.Equal(t, f.A.ID, *u.BuildingID)
	})
}

func TestAuth_ListUsersScope(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	all := app.Client.Do(http.MethodGet, "/api/admin/auth/users", f.TokenRoot, nil)
	all.RequireOK(t)
	var allData struct {
		Users []struct {
			Role     string `json:"role"`
			Building *uint  `json:"building_id"`
		} `json:"users"`
	}
	all.Decode(t, &allData)
	require.GreaterOrEqual(t, len(allData.Users), 7)
	require.NotContains(t, string(all.Raw), "password_hash")

	own := app.Client.Do(http.MethodGet, "/api/building/auth/users", f.TokenA, nil)
	own.RequireOK(t)
	var ownData struct {
		Users []struct {
			Username   string `json:"username"`
			BuildingID *uint  `json:"building_id"`
		} `json:"users"`
	}
	own.Decode(t, &ownData)
	require.NotEmpty(t, ownData.Users)
	for _, u := range ownData.Users {
		// ListBuildingUsers 只 select id/username/role/created_at，不返回 building_id
		require.NotEqual(t, "badmin", u.Username)
		require.NotEqual(t, "cadmin", u.Username)
		require.NotEqual(t, "dadmin", u.Username)
	}
	var aUser models.User
	require.NoError(t, app.DB.Where("username = ?", "aadmin").First(&aUser).Error)
	require.NotNil(t, aUser.BuildingID)
	require.Equal(t, f.A.ID, *aUser.BuildingID)
}

func TestAuth_UpdatePasswordInvalidatesToken(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	// 用 aadmin2 当前 token
	before := app.Client.Do(http.MethodGet, "/api/building/info", f.TokenAPlain, nil)
	before.RequireOK(t)

	up := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/auth/users/%d", f.APlain.ID), f.TokenRoot, map[string]any{
		"password": "NewPass123",
	})
	up.RequireOK(t)

	old := app.Client.Do(http.MethodGet, "/api/building/info", f.TokenAPlain, nil)
	old.RequireError(t, http.StatusUnauthorized, 401)

	lr := login(t, app.Client, "aadmin2", "NewPass123")
	lr.RequireOK(t)
}

func TestAuth_DeleteUser(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	u := models.User{Username: "del_user_01", PasswordHash: testkit.HashFor("Test12345"), Role: "admin", IsActive: true, BuildingID: &f.A.ID}
	require.NoError(t, app.DB.Create(&u).Error)

	r := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/auth/users/%d", u.ID), f.TokenRoot, nil)
	r.RequireOK(t)

	r2 := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/auth/users/%d", u.ID), f.TokenRoot, nil)
	r2.RequireError(t, http.StatusNotFound, 404)

	super := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/admin/auth/users/%d", f.Root.ID), f.TokenRoot, nil)
	super.RequireError(t, http.StatusForbidden, 403)
}

func TestAuth_LoginRateLimit(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	ip := "10.250.250.250"
	c := testkit.NewClient(t, app.Server.URL, ip)
	limit := 10
	if v := os.Getenv("LOGIN_RATE_LIMIT_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	for i := 0; i < limit; i++ {
		r := login(t, c, "root", "wrong")
		r.RequireError(t, http.StatusUnauthorized, 401)
	}
	r := login(t, c, "root", "root")
	r.RequireError(t, http.StatusTooManyRequests, 429)
}

func TestAuth_SuccessfulLoginNotRateLimited(t *testing.T) {
	app := testkit.NewApp(t)
	app.SeedBase(t)

	c := testkit.NewClient(t, app.Server.URL, "10.250.250.251")
	for i := 0; i < 12; i++ {
		r := login(t, c, "root", "root")
		r.RequireOK(t)
	}
}
