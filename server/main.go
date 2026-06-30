package main

import (
	"fmt"
	"os"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/utils"
	"rental-server/handlers"
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
	logger.Log.Info().Int("max_idle", 10).Int("max_open", 100).Msg("数据库连接池已配置")

	if os.Getenv("GIN_MODE") == "release" {
		logger.Log.Warn().Msg("生产环境 AutoMigrate 可能修改表结构，建议手动管理迁移")
	}
	if err := models.AutoMigrate(db); err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库迁移失败")
	}
	logger.Log.Info().Msg("数据库迁移完成")

	utils.InitBillNo(db)
	seedAdmin(db)

	// 启动时自动创建当月租金账单
	handlers.AutoCreateMonthlyRentBills(db)
	// 定时任务：每月1号自动创建租金账单
	go func() {
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
		for {
			checkExpiredBuildings(db)
			time.Sleep(6 * time.Hour)
		}
	}()

	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
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
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
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
		c.Next()
	})
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
		hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
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
		logger.Log.Info().Msg("已创建默认超级管理员: admin / admin123")
	} else {
		if admin.Role != "super_admin" {
			db.Model(&admin).Update("role", "super_admin")
			logger.Log.Info().Uint("user_id", admin.ID).Msg("已升级 admin 为超级管理员")
		}
	}
}
