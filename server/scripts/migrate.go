package main

import (
	"crypto/rand"
	"fmt"
	"math/big"

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

	seedAdmin(db)
}

func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "admin").First(&admin)
	if result.Error != nil {
		randomPassword := generateRandomPassword(16)
		hash, _ := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		admin = models.User{
			Username:     "admin",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		db.Create(&admin)
		fmt.Printf("已创建默认超级管理员: admin / %s\n", randomPassword)
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
