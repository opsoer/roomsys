package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
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

	if os.Getenv("GIN_MODE") == "release" {
		logger.Log.Warn().Msg("生产环境跳过 AutoMigrate，请使用版本化迁移工具管理表结构")
	} else {
		if err := models.AutoMigrate(db); err != nil {
			logger.Log.Fatal().Err(err).Msg("数据库迁移失败")
		}
		logger.Log.Info().Msg("数据库迁移完成")
	}

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

	// 每天凌晨3点清理超过90天的软删除数据
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error().Interface("panic", r).Msg("软删除清理定时任务 panic，已恢复")
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
		}
	}()

	if err := os.MkdirAll(cfg.UploadDir, 0750); err != nil {
		logger.Log.Fatal().Err(err).Str("dir", cfg.UploadDir).Msg("创建上传目录失败")
	}
	logger.Log.Info().Str("dir", cfg.UploadDir).Msg("上传目录就绪")

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
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; media-src 'self' https:; font-src 'self'")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if os.Getenv("GIN_MODE") == "release" {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	})

	r.Use(middleware.RateLimitMiddleware())

	routes.Setup(r, db, cfg)
	logger.Log.Info().Str("port", cfg.ServerPort).Msg("服务启动")
	r.Run(":" + cfg.ServerPort)
}

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

func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)
	if result.Error != nil {
		randomPassword := generateRandomPassword(16)
		hash, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		if err != nil {
			logger.Log.Fatal().Err(err).Msg("管理员密码加密失败")
			return
		}
		admin = models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		if err := db.Create(&admin).Error; err != nil {
			logger.Log.Fatal().Err(err).Msg("创建默认管理员失败")
			return
		}
		logger.Log.Info().Str("password", randomPassword).Msg("已创建默认超级管理员: admin，请登录后立即修改密码")
	} else {
		if admin.Role != "super_admin" {
			db.Model(&admin).Update("role", "super_admin")
			logger.Log.Info().Uint("user_id", admin.ID).Msg("已升级 admin 为超级管理员")
		}
	}
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		result[i] = charset[n.Int64()]
	}
	return string(result)
}

func splitStr(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
