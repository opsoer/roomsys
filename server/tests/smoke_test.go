package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/tests/testkit"
)

func TestSmoke_HealthAndFixture(t *testing.T) {
	app := testkit.NewApp(t)
	f := app.SeedBase(t)

	r := app.Client.Do(http.MethodGet, "/api/config", "", nil)
	r.RequireOK(t)

	var cfg struct {
		CDNDomain string `json:"cdn_domain"`
		CDNScheme string `json:"cdn_scheme"`
	}
	r.Decode(t, &cfg)
	require.Empty(t, cfg.CDNDomain)

	// fixture 数量
	var buildings, rooms, contracts, bills, shareholders, dividends, tasks, users int64
	require.NoError(t, app.DB.Model(&struct{}{}).Table("buildings").Count(&buildings).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("rooms").Count(&rooms).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("rental_contracts").Count(&contracts).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("bills").Where("deleted_at IS NULL").Count(&bills).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("shareholders").Count(&shareholders).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("dividends").Count(&dividends).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("tasks").Count(&tasks).Error)
	require.NoError(t, app.DB.Model(&struct{}{}).Table("users").Count(&users).Error)

	require.EqualValues(t, 4, buildings)
	require.EqualValues(t, 11, rooms)
	require.EqualValues(t, 6, contracts)
	require.EqualValues(t, 8, bills)
	require.EqualValues(t, 2, shareholders)
	require.EqualValues(t, 2, dividends)
	require.EqualValues(t, 1, tasks)
	require.EqualValues(t, 7, users)

	// root 登录
	login := app.Client.Do(http.MethodPost, "/api/auth/login", "", map[string]any{
		"username": "root", "password": "root",
	})
	login.RequireOK(t)
	var data struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
		User         struct {
			Role string `json:"role"`
		} `json:"user"`
	}
	login.Decode(t, &data)
	require.NotEmpty(t, data.Token)
	require.NotEmpty(t, data.RefreshToken)
	require.Equal(t, "super_admin", data.User.Role)

	// 用 token 访问平台接口
	b := app.Client.Do(http.MethodGet, "/api/admin/buildings", data.Token, nil)
	b.RequireOK(t)
	var list struct {
		Total int `json:"total"`
	}
	b.Decode(t, &list)
	require.Equal(t, 4, list.Total)

	_ = f
}
