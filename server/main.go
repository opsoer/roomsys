// main 是应用程序入口，负责初始化配置、数据库、路由并启动 HTTP 服务。
package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/utils"
	"rental-server/handlers"
	"rental-server/middleware"
	"rental-server/models"
	"rental-server/routes"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// main 是程序入口：加载配置、初始化日志、连接数据库、设置定时任务和路由。
func main() {
	cfg := config.Load()

	logger.Init(logger.Config{
		Level: cfg.LogLevel,
		Dir:   cfg.LogDir,
	})
	logger.Log.Info().Str("level", cfg.LogLevel).Str("dir", cfg.LogDir).Msg("日志系统初始化完成")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库连接失败")
	}
	logger.Log.Info().Msg("数据库连接成功")

	sqlDB, err := db.DB()
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("获取数据库连接实例失败")
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	logger.Log.Info().Int("max_idle", 10).Int("max_open", 100).Msg("数据库连接池已配置")

	if os.Getenv("RESET_DATABASE") == "true" {
		logger.Log.Warn().Msg("检测到 RESET_DATABASE=true，将删除所有表并重建")
		resetDatabase(db)
	}

	if err := models.AutoMigrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库迁移失败")
	}
	logger.Log.Info().Msg("数据库迁移完成")

	seedAdmin(db)

	// 启动时自动创建当月租金账单
	handlers.AutoCreateMonthlyRentBills(db)
	// 定时任务：每月1号自动创建租金账单
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error().Interface("panic", r).Msg("租金账单定时任务 panic，已恢复")
			}
		}()
		for {
			now := utils.Now()
			next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
			dur := next.Sub(now)
			if dur <= 0 {
				dur = time.Hour
			}
			time.Sleep(dur)
			handlers.AutoCreateMonthlyRentBills(db)
		}
	}()

	// 每6小时检查一次公寓到期
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error().Interface("panic", r).Msg("到期检查定时任务 panic，已恢复")
			}
		}()
		for {
			checkExpiredBuildings(db)
			time.Sleep(6 * time.Hour)
		}
	}()

	// 每12小时清理已吊销的令牌
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error().Interface("panic", r).Msg("令牌清理定时任务 panic，已恢复")
			}
		}()
		for {
			time.Sleep(12 * time.Hour)
			utils.CleanupRevokedTokens()
		}
	}()

	// 每天凌晨3点清理超过90天的软删除数据和page_views
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error().Interface("panic", r).Msg("清理定时任务 panic，已恢复")
			}
		}()
		for {
			now := utils.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, now.Location())
			time.Sleep(next.Sub(now))
			if err := models.CleanupSoftDeleted(db, 90); err != nil {
				logger.Log.Error().Err(err).Msg("软删除数据清理失败")
			} else {
				logger.Log.Info().Msg("软删除数据清理完成")
			}
			cutoff := time.Now().AddDate(0, 0, -90)
			if err := db.Where("created_at < ?", cutoff).Delete(&models.PageView{}).Error; err != nil {
				logger.Log.Error().Err(err).Msg("page_views 清理失败")
			} else {
				logger.Log.Info().Msg("page_views 清理完成")
			}
		}
	}()

	if err := os.MkdirAll(cfg.UploadDir, 0750); err != nil {
		logger.Log.Fatal().Err(err).Str("dir", cfg.UploadDir).Msg("创建上传目录失败")
	}
	logger.Log.Info().Str("dir", cfg.UploadDir).Msg("上传目录就绪")

	ensureFFmpegCore(cfg.UploadDir)

	if os.Getenv("GIN_MODE") != "debug" {
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
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; media-src 'self' https:; font-src 'self'; worker-src 'self' blob:")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if os.Getenv("GIN_MODE") == "release" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	})

	r.Use(gzipMiddleware())
	r.Use(cacheControlMiddleware())

	r.Use(middleware.RateLimitMiddleware())

	// 初始化统计写入器
	utils.InitStatsWriter(db)

	routes.Setup(r, db, cfg)
	logger.Log.Info().Str("port", cfg.ServerPort).Msg("服务启动")
	r.Run(":" + cfg.ServerPort)
}

// requestLogger 返回 Gin 中间件，记录每个 HTTP 请求的方法、路径、状态码和耗时。
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

		lvl := zerolog.InfoLevel
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

// checkExpiredBuildings 检查所有到期公寓，将已过期的状态更新为 expired。
func checkExpiredBuildings(db *gorm.DB) {
	var buildings []models.Building
	db.Where("status = ? AND expired_at IS NOT NULL AND expired_at != ''", "active").Find(&buildings)
	now := utils.Now()
	expiredCount := 0
	for _, b := range buildings {
		if expDate, err := time.Parse("2006-01-02", b.ExpiredAt); err == nil {
			if now.After(expDate) {
				db.Model(&b).Update("status", "expired")
				expiredCount++
				logger.Log.Info().
					Uint("building_id", b.ID).
					Str("name", b.Name).
					Str("expired_at", b.ExpiredAt).
					Msg("公寓已到期，状态更新为 expired")
			}
		}
	}
	if expiredCount > 0 {
		logger.Log.Info().Int("count", expiredCount).Msg("到期公寓检查完成")
	} else {
		logger.Log.Debug().Msg("到期公寓检查完成，无到期公寓")
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
		&models.Building{},
		&models.User{},
		&models.PageView{},
	}
	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			logger.Log.Warn().Err(err).Msg("删除表失败")
		}
	}
	logger.Log.Info().Msg("所有表已删除，等待 AutoMigrate 重建")
}

// seedAdmin 检查是否存在 admin 用户，如不存在则创建，否则重置密码。
func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)
	password := "admin123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("管理员密码加密失败")
		return
	}

	if result.Error != nil {
		admin = models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		if err := db.Create(&admin).Error; err != nil {
			logger.Log.Fatal().Err(err).Msg("创建默认管理员失败")
			return
		}
		logger.Log.Info().Msg("已创建默认超级管理员: admin / admin")
	} else {
		db.Model(&admin).Updates(map[string]interface{}{
			"password_hash": string(hash),
			"role":          "super_admin",
		})
		logger.Log.Info().Msg("已重置超级管理员密码: admin / admin")
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
	writer *gzip.Writer
}

// Write 使用 gzip.Writer 压缩后写入响应。
func (g *gzipWriter) Write(data []byte) (int, error) {
	return g.writer.Write(data)
}

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
		c.Writer = &gzipWriter{ResponseWriter: c.Writer, writer: gz}
		c.Next()
		gz.Close()
		// 如果响应为空，去掉 Content-Encoding 头防止浏览器误解
		if c.Writer.Size() == 0 {
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
		// 首页 HTML：不缓存，保证发布后用户拿到最新
		if path == "/" || path == "" {
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
