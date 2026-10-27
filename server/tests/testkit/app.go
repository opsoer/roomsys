package testkit

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"rental-server/audit"
	"rental-server/config"
	"rental-server/database"
	"rental-server/handlers"
	"rental-server/logger"
	"rental-server/middleware"
	"rental-server/migrations"
	"rental-server/models"
	"rental-server/routes"
	"rental-server/scheduler"
	"rental-server/utils"
)

// App 进程内测试应用：真实路由 + 真实中间件 + 临时 SQLite。
type App struct {
	T      *testing.T
	DB     *gorm.DB
	Cfg    *config.Config
	Engine *gin.Engine
	Server *httptest.Server
	Client *Client
	IP     string
	Dir    string
}

var (
	hashMu    sync.Mutex
	hashCache = map[string]string{}
)

// HashFor 生成（并缓存）测试密码哈希。使用 MinCost，避免每个 fixture 重复做 bcrypt。
func HashFor(password string) string {
	hashMu.Lock()
	defer hashMu.Unlock()
	if h, ok := hashCache[password]; ok {
		return h
	}
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		panic(err)
	}
	hashCache[password] = string(h)
	return string(h)
}

// NewApp 建立独立测试应用。调用方无需关闭，t.Cleanup 会处理。
func NewApp(t *testing.T) *App {
	t.Helper()

	gin.SetMode(gin.TestMode)
	logger.Log = zerolog.Nop()
	utils.InitRevealQuota(5)
	utils.SetTimeOffset(0)
	utils.ClearStatsCache()

	dir := t.TempDir()
	cfg := testConfig(dir)
	db, err := database.Open(cfg)
	require.NoError(t, err, "打开测试数据库失败")
	_, err = migrations.Apply(db)
	require.NoError(t, err, "执行迁移失败")

	root := models.User{
		Username:     "root",
		PasswordHash: HashFor("root"),
		Role:         "super_admin",
		IsActive:     true,
	}
	require.NoError(t, db.Create(&root).Error, "创建 root 失败")
	audit.Init(db)

	engine := gin.New()
	engine.Use(gin.Recovery())
	require.NoError(t, engine.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"}))
	engine.Use(middleware.RateLimitMiddleware())

	jobs := newJobs(db, cfg)
	routes.Setup(engine, db, cfg, jobs)

	srv := httptest.NewServer(engine)
	app := &App{
		T:      t,
		DB:     db,
		Cfg:    cfg,
		Engine: engine,
		Server: srv,
		IP:     uniqueIP(),
		Dir:    dir,
	}
	app.Client = NewClient(t, srv.URL, app.IP)

	t.Cleanup(func() {
		srv.Close()
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		utils.SetTimeOffset(0)
		utils.ClearStatsCache()
	})
	return app
}

func testConfig(dir string) *config.Config {
	webDir := filepath.Join(dir, "webdist")
	requireNoErr(os.MkdirAll(filepath.Join(webDir, "assets"), 0o755))
	requireNoErr(os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<!doctype html><html><body>test</body></html>"), 0o644))
	requireNoErr(os.WriteFile(filepath.Join(webDir, "default-image.svg"), []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), 0o644))
	requireNoErr(os.MkdirAll(filepath.Join(dir, "media", "ffmpeg"), 0o755))
	requireNoErr(os.MkdirAll(filepath.Join(dir, "logs"), 0o755))

	return &config.Config{
		Server: config.ServerConfig{
			Port:             "0",
			JWTSecret:        "roomsys-test-secret",
			UploadDir:        filepath.Join(dir, "media"),
			LogLevel:         "error",
			LogDir:           filepath.Join(dir, "logs"),
			WebDistDir:       webDir,
			RevealFreePerDay: 5,
		},
		Database: config.DatabaseConfig{
			Driver: config.DriverSQLite,
			SQLite: config.SQLiteConfig{Path: filepath.Join(dir, "rental.db")},
		},
		Storage: config.StorageConfig{
			MediaMaxAge: 1,
		},
		Scheduler: config.SchedulerConfig{
			RentBills:         config.JobConfig{Enabled: true, Time: "03:00"},
			BuildingExpiry:    config.JobConfig{Enabled: true, Time: "03:00"},
			ContractExpiry:    config.JobConfig{Enabled: true, Time: "03:00"},
			SoftDeleteCleanup: config.JobConfig{Enabled: true, Time: "03:00"},
			TokenCleanup:      config.JobConfig{Enabled: true, IntervalHours: 12},
			DBBackup:          config.JobConfig{Enabled: false, Time: "04:00", Keep: 7},
		},
	}
}

func requireNoErr(err error) {
	if err != nil {
		panic(err)
	}
}

func newJobs(db *gorm.DB, cfg *config.Config) *scheduler.Registry {
	r := scheduler.NewRegistry()
	r.Add(scheduler.NewJob("rent_bills", "月度租金账单生成", cfg.Scheduler.RentBills, false,
		func() string { return handlers.AutoCreateMonthlyRentBills(db) }))
	r.Add(scheduler.NewJob("building_expiry", "公寓到期检查", cfg.Scheduler.BuildingExpiry, false,
		func() string { return scheduler.CheckExpiredBuildings(db) }))
	r.Add(scheduler.NewJob("contract_expiry", "到期合同检查", cfg.Scheduler.ContractExpiry, false,
		func() string { return scheduler.AutoCheckExpiringContracts(db) }))
	r.Add(scheduler.NewJob("soft_delete_cleanup", "软删除清理", cfg.Scheduler.SoftDeleteCleanup, false,
		func() string { return scheduler.AutoCleanupData(db) }))
	r.Add(scheduler.NewJob("token_cleanup", "吊销令牌清理", cfg.Scheduler.TokenCleanup, false,
		func() string { return utils.CleanupRevokedTokens() }))
	r.Add(scheduler.NewJob("db_backup", "数据库备份到对象存储", cfg.Scheduler.DBBackup, false,
		func() string { return "测试环境不执行对象存储备份" }))
	return r
}

func uniqueIP() string {
	return fmt.Sprintf("10.%d.%d.%d", 1+rand.Intn(250), 1+rand.Intn(250), 1+rand.Intn(250))
}

// TokenFor 直接用测试 secret 签发 token，避免每个用例都走登录限流。
func (a *App) TokenFor(user models.User) string {
	bid := uint(0)
	if user.BuildingID != nil {
		bid = *user.BuildingID
	}
	token, err := utils.GenerateToken(user.ID, user.Username, user.Role, a.Cfg.Server.JWTSecret, bid)
	require.NoError(a.T, err)
	return token
}

// SetTime 设置全局模拟时间，并在用例结束时自动重置。
func (a *App) SetTime(t *testing.T, target time.Time) {
	t.Helper()
	utils.SetTimeOffset(target.Sub(time.Now()))
	t.Cleanup(func() { utils.SetTimeOffset(0) })
}

// ResetTime 立即重置模拟时间。
func (a *App) ResetTime() {
	utils.SetTimeOffset(0)
}

// NewDB 供需要独立数据库的用例使用（如迁移/备份）。
func NewDB(t *testing.T) (*gorm.DB, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	cfg := testConfig(dir)
	db, err := database.Open(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db, cfg
}

// decodeJSON 是 client 内部使用的宽松解码。
func decodeJSON(raw []byte, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// ConfigForDir 返回指向指定目录的测试配置（会创建 webdist/media/logs 目录）。
func ConfigForDir(dir string) *config.Config { return testConfig(dir) }
