// main 是应用程序入口，负责初始化配置、数据库、路由并启动 HTTP 服务。
package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Version 编译期注入的版本号（CI 构建时写入 commit sha），用于确认线上跑的是哪一版。
var Version = "dev"

// main 是程序入口：加载配置、初始化日志、连接数据库、设置定时任务和路由。
func main() {
	cfg := config.Load()

	logger.Init(logger.Config{
		Level: cfg.Server.LogLevel,
		Dir:   cfg.Server.LogDir,
	})
	logger.Log.Info().Str("level", cfg.Server.LogLevel).Str("dir", cfg.Server.LogDir).Msg("日志系统初始化完成")

	db, err := database.Open(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库连接失败")
	}
	logger.Log.Info().Str("driver", cfg.Database.Driver).Msg("数据库连接成功")

	// 审计模块需要数据库句柄：写操作审计既写日志文件，也落库到 audit_logs 表
	audit.Init(db)

	if os.Getenv("RESET_DATABASE") == "true" {
		logger.Log.Warn().Msg("检测到 RESET_DATABASE=true，将删除所有表并重建")
		resetDatabase(db)
	}

	// 数据库结构迁移：按编号顺序执行尚未应用的版本（台账表 schema_migrations）
	migrated, err := migrations.Apply(db)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库迁移失败")
	}
	if len(migrated) > 0 {
		logger.Log.Info().Ints("versions", migrated).Msg("数据库结构已更新")
	} else {
		logger.Log.Info().Msg("数据库结构已是最新")
	}

	seedAdmin(db)

	// 定时任务注册表：定义/周期来自 config.yaml scheduler 段，统一调度与手动执行共用一份注册
	jobs := scheduler.NewRegistry()
	jobs.Add(scheduler.NewJob("rent_bills", "月度租金账单生成", cfg.Scheduler.RentBills, true,
		func() string { return handlers.AutoCreateMonthlyRentBills(db) }))
	jobs.Add(scheduler.NewJob("building_expiry", "公寓到期检查", cfg.Scheduler.BuildingExpiry, true,
		func() string { return scheduler.CheckExpiredBuildings(db) }))
	jobs.Add(scheduler.NewJob("contract_expiry", "到期合同检查", cfg.Scheduler.ContractExpiry, true,
		func() string { return scheduler.AutoCheckExpiringContracts(db) }))
	jobs.Add(scheduler.NewJob("soft_delete_cleanup", "软删除清理", cfg.Scheduler.SoftDeleteCleanup, false,
		func() string { return scheduler.AutoCleanupData(db) }))
	jobs.Add(scheduler.NewJob("token_cleanup", "吊销令牌清理", cfg.Scheduler.TokenCleanup, false,
		utils.CleanupRevokedTokens))
	jobs.Add(scheduler.NewJob("db_backup", "数据库备份到对象存储", cfg.Scheduler.DBBackup, false,
		func() string { return scheduler.BackupDatabase(db, cfg) }))
	jobs.Start()

	if err := os.MkdirAll(cfg.Server.UploadDir, 0750); err != nil {
		logger.Log.Fatal().Err(err).Str("dir", cfg.Server.UploadDir).Msg("创建上传目录失败")
	}
	logger.Log.Info().Str("dir", cfg.Server.UploadDir).Msg("上传目录就绪")

	ensureFFmpegCore(cfg.Server.UploadDir)

	if gin.Mode() != gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.MaxMultipartMemory = 200 << 20 // 200MB

	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := []string{
			"http://localhost:5173",
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
		if extraOrigins := os.Getenv("CORS_ORIGINS"); extraOrigins != "" {
			for _, o := range splitStr(extraOrigins, ",") {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
		allowed := false
		for _, o := range allowedOrigins {
			if origin == o {
				allowed = true
				break
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Requested-With, X-CSRF-Token")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; media-src 'self' https:; font-src 'self' data:; worker-src 'self' blob:; connect-src 'self' https://*.qiniup.com https://*.qiniudn.com")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if gin.Mode() == gin.ReleaseMode {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	})

	r.Use(gzipMiddleware())
	r.Use(cacheControlMiddleware())

	r.Use(middleware.RateLimitMiddleware())

	// 初始化统计写入器
	utils.InitStatsWriter(db)

	routes.Setup(r, db, cfg, jobs)
	logger.Log.Info().Str("port", cfg.Server.Port).Str("version", Version).Msg("服务启动")
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Log.Fatal().Err(err).Msg("HTTP 服务启动失败")
	}
}

// requestLogger 返回 Gin 中间件，记录每个 HTTP 请求的方法、路径、状态码和耗时。
//
// 分级策略（写操作审计由 audit 包单独记录，这里只做请求轨迹）：
//   - 2xx/3xx：查询与读请求，默认不输出（降为 Debug），避免访问日志淹没关键信息；
//   - 4xx：客户端错误，记 Warn（越权、参数错误、限流等）；
//   - 5xx：服务端错误，记 Error。
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		lvl := zerolog.DebugLevel
		if status >= 500 {
			lvl = zerolog.ErrorLevel
		} else if status >= 400 {
			lvl = zerolog.WarnLevel
		}

		logger.Log.WithLevel(lvl).
			Str("method", method).
			Str("path", path).
			Str("query", query).
			Int("status", status).
			Str("ip", clientIP).
			Dur("latency", latency).
			Msg("")
	}
}

// resetDatabase 删除所有表，供 AutoMigrate 重建。
func resetDatabase(db *gorm.DB) {
	tables := []interface{}{
		&models.AuditLog{},
		&models.RecruitSubmission{},
		&models.Setting{},
		&models.Task{},
		&models.Dividend{},
		&models.Shareholder{},
		&models.Bill{},
		&models.RentalContract{},
		&models.Tenant{},
		&models.RoomMedia{},
		&models.Room{},
		&models.BuildingLandlord{},
		&models.BuildingRenewal{},
		&models.Building{},
		&models.User{},
		&models.PageView{},
		&models.LocationCustom{},
		&models.LocationRename{},
	}
	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			logger.Log.Warn().Err(err).Msg("删除表失败")
		}
	}
	logger.Log.Info().Msg("所有表已删除，等待 AutoMigrate 重建")
}

// seedAdmin 检查是否存在 root 用户，如不存在则创建，否则重置密码。
func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "root").First(&admin)
	password := "root"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("管理员密码加密失败")
		return
	}

	if result.Error != nil {
		admin = models.User{
			Username:     "root",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		if err := db.Create(&admin).Error; err != nil {
			logger.Log.Fatal().Err(err).Msg("创建默认管理员失败")
			return
		}
		logger.Log.Info().Msg("已创建默认超级管理员: root / root")
	} else {
		db.Model(&admin).Updates(map[string]interface{}{
			"password_hash": string(hash),
			"role":          "super_admin",
		})
		logger.Log.Info().Msg("已重置超级管理员密码: root / root")
	}
}

// ensureFFmpegCore 确保 FFmpeg WASM 核心文件已下载到上传目录。
func ensureFFmpegCore(uploadDir string) {
	dir := filepath.Join(uploadDir, "ffmpeg")
	if err := os.MkdirAll(dir, 0750); err != nil {
		logger.Log.Warn().Err(err).Msg("创建 ffmpeg 目录失败")
		return
	}

	baseURL := "https://cdn.jsdelivr.net/npm/@ffmpeg/core@0.12.10/dist/esm"
	files := []string{"ffmpeg-core.js", "ffmpeg-core.wasm"}

	for _, f := range files {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); err == nil {
			logger.Log.Info().Str("file", f).Msg("FFmpeg core 文件已存在，跳过下载")
			continue
		}

		url := baseURL + "/" + f
		logger.Log.Info().Str("file", f).Msg("正在下载 FFmpeg core 文件")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			logger.Log.Warn().Err(err).Str("url", url).Msg("下载 FFmpeg core 失败")
			continue
		}

		out, err := os.Create(path)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("创建 ffmpeg 文件失败")
			resp.Body.Close()
			continue
		}

		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			logger.Log.Warn().Err(err).Msg("写入 ffmpeg 文件失败")
			os.Remove(path)
		} else {
			logger.Log.Info().Str("file", f).Msg("FFmpeg core 文件下载完成")
		}
	}
}

// gzipWriter 包装 gin.ResponseWriter，用 gzip 压缩响应体。
type gzipWriter struct {
	gin.ResponseWriter
	writer  *gzip.Writer
	written int64
}

// Write 使用 gzip.Writer 压缩后写入响应，同时记录原始写入字节数。
// 不能依赖内嵌 ResponseWriter 的 Size()：经本包装后它无法反映 handler 是否实际写出内容。
func (g *gzipWriter) Write(data []byte) (int, error) {
	g.written += int64(len(data))
	return g.writer.Write(data)
}

// skipGzipPrefixes 不需要 gzip 压缩的路径前缀（媒体文件和 FFmpeg wasm 已压缩过）
var skipGzipPrefixes = []string{"/api/media/", "/api/ffmpeg/"}

// gzipMiddleware 返回 Gin 中间件，对符合条件的请求启用 gzip 压缩。
func gzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		// 图片/视频/FFmpeg wasm 已经压缩过，跳过
		for _, p := range skipGzipPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}
		gz, err := gzip.NewWriterLevel(c.Writer, gzip.DefaultCompression)
		if err != nil {
			c.Next()
			return
		}
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		gw := &gzipWriter{ResponseWriter: c.Writer, writer: gz}
		c.Writer = gw
		c.Next()
		gz.Close()
		// 如果响应为空，去掉 Content-Encoding 头防止浏览器误解
		if gw.written == 0 {
			c.Header("Content-Encoding", "")
		}
	}
}

// cacheControlMiddleware 返回 Gin 中间件，设置静态资源的缓存策略。
func cacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 哈希文件名的静态资源：缓存 1 年
		if strings.HasPrefix(path, "/assets/") && strings.Contains(path, "-") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		// HTML 文档（SPA 全部前端路由）：不缓存，保证发布后用户拿到最新
		// 仅 /assets/ 与 /api/ 例外（前者已设长缓存，后者是接口与媒体文件）
		if !strings.HasPrefix(path, "/assets/") && !strings.HasPrefix(path, "/api/") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		c.Next()
	}
}

// splitStr 按分隔符分割字符串，并去除空白项。
func splitStr(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
