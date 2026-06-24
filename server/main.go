package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"rental-server/config"
	"rental-server/utils"
	"rental-server/handlers"
	"rental-server/models"
	"rental-server/routes"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}

	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	seedAdmin(db)

	// 启动时自动创建当月租金账单
	handlers.AutoCreateMonthlyRentBills(db)
	// 定时任务：每月1号自动创建租金账单
	go func() {
		for {
			now := utils.Now()
			next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
			time.Sleep(next.Sub(now))
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
		log.Fatalf("创建上传目录失败: %v", err)
	}

	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()
	r.MaxMultipartMemory = 20 << 20 // 20MB
	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	})
	routes.Setup(r, db, cfg)
	r.Run(":" + cfg.ServerPort)
}

func checkExpiredBuildings(db *gorm.DB) {
	var buildings []models.Building
	db.Where("status = ? AND expired_at IS NOT NULL AND expired_at != ''", "active").Find(&buildings)
	now := utils.Now()
	for _, b := range buildings {
		if expDate, err := time.Parse("2006-01-02", b.ExpiredAt); err == nil {
			if now.After(expDate) {
				db.Model(&b).Update("status", "expired")
			}
		}
	}
}

func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)
	if result.Error != nil {
		// 创建默认超级管理员
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin = models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		db.Create(&admin)
		log.Println("已创建默认超级管理员: admin / admin123")
	} else {
		if admin.Role != "super_admin" {
			db.Model(&admin).Update("role", "super_admin")
			log.Println("已升级 admin 为超级管理员")
		}
	}
}
