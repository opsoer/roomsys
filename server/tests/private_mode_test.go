package tests

import (
	"net/http"
	"testing"

	"rental-server/config"
	"rental-server/tests/testkit"

	"github.com/stretchr/testify/require"
)

// 私享模式（private）：平台级公开接口仅超级管理员可用，
// 公寓详情类接口保持公开（访客通过链接直达），/api/config 下发模式。
func TestPrivateMode_Guard(t *testing.T) {
	app, f := testkit.NewAppWithFixtureOptions(t, func(cfg *config.Config) {
		cfg.Server.Mode = config.ModePrivate
	})

	// 平台级接口：匿名 / 普通用户一律 404（不暴露接口存在性）
	guarded := []struct{ method, path string }{
		{http.MethodGet, "/api/buildings"},
		{http.MethodGet, "/api/buildings/locations"},
		{http.MethodGet, "/api/locations/options"},
		{http.MethodPost, "/api/recruit/submit"},
	}
	for _, g := range guarded {
		t.Run("anonymous "+g.method+" "+g.path, func(t *testing.T) {
			app.Client.Do(g.method, g.path, "", nil).RequireError(t, http.StatusNotFound, 404)
		})
		t.Run("building_admin "+g.method+" "+g.path, func(t *testing.T) {
			app.Client.Do(g.method, g.path, f.TokenA, nil).RequireError(t, http.StatusNotFound, 404)
		})
	}

	// 超级管理员令牌：正常访问
	r := app.Client.Do(http.MethodGet, "/api/buildings", f.TokenRoot, nil)
	r.RequireOK(t)

	// 公寓详情保持公开：访客通过链接直达
	app.Client.Do(http.MethodGet, "/api/buildings/"+itoa(f.A.ID), "", nil).RequireOK(t)
}

func TestPrivateMode_Config(t *testing.T) {
	app, f := testkit.NewAppWithFixtureOptions(t, func(cfg *config.Config) {
		cfg.Server.Mode = config.ModePrivate
	})

	var data struct {
		Mode            string `json:"mode"`
		DefaultBuilding *struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		} `json:"default_building"`
	}

	// 模式字段必须下发
	r := app.Client.Do(http.MethodGet, "/api/config", "", nil)
	r.RequireOK(t)
	r.Decode(t, &data)
	require.Equal(t, config.ModePrivate, data.Mode)

	// fixture 初始有 A、B、C、D 四栋可见公寓 → 不下发 default_building
	require.Nil(t, data.DefaultBuilding)

	// 只剩 1 栋可见公寓时，下发它作为默认直达目标（软删除其余公寓）
	require.NoError(t, app.DB.Delete(&f.B).Error)
	require.NoError(t, app.DB.Delete(&f.C).Error)
	require.NoError(t, app.DB.Delete(&f.D).Error)
	r2 := app.Client.Do(http.MethodGet, "/api/config", "", nil)
	r2.RequireOK(t)
	r2.Decode(t, &data)
	require.NotNil(t, data.DefaultBuilding)
	require.Equal(t, f.A.ID, data.DefaultBuilding.ID)
}

// 平台模式：同一批公开接口对匿名保持开放（守卫直通，现状行为不变）。
func TestPlatformMode_PublicRoutesUnchanged(t *testing.T) {
	app, _ := testkit.NewAppWithFixtureOptions(t, nil)

	app.Client.Do(http.MethodGet, "/api/buildings", "", nil).RequireOK(t)
	app.Client.Do(http.MethodGet, "/api/buildings/locations", "", nil).RequireOK(t)
	app.Client.Do(http.MethodGet, "/api/locations/options", "", nil).RequireOK(t)

	cfg := app.Client.Do(http.MethodGet, "/api/config", "", nil)
	cfg.RequireOK(t)
	require.Contains(t, string(cfg.Raw), `"mode":"platform"`)
}
