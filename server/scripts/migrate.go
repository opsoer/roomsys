package main

import (
	"fmt"

	"rental-server/config"
	"rental-server/models"

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
		panic(err)
	}

	if err := models.AutoMigrate(db); err != nil {
		panic(err)
	}
	fmt.Println("数据库迁移完成")

	fmt.Println("唯一索引已确认")

	seedAdmin(db)
	fmt.Println("默认管理员已创建: admin / admin123")
}

func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)
	if result.Error != nil {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin = models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		db.Create(&admin)
	}
}
