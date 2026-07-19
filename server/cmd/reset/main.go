package main

import (
	"fmt"
	"os"
	"path/filepath"

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

	fmt.Println("1. 删除除数据面板(page_views)外的所有业务表...")
	db.Migrator().DropTable(
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
	)
	fmt.Println("   完成")

	fmt.Println("2. 重建表结构...")
	if err := models.AutoMigrate(db); err != nil {
		panic(err)
	}
	fmt.Println("   完成")

	fmt.Println("3. 清空数据面板统计(page_views)数据...")
	if err := db.Where("1 = 1").Delete(&models.PageView{}).Error; err != nil {
		panic(err)
	}
	fmt.Println("   完成")

	fmt.Println("4. 创建默认超级管理员...")
	password := "admin123"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	admin := models.User{
		Username:     "admin",
		PasswordHash: string(hash),
		Role:         "super_admin",
	}
	if err := db.Create(&admin).Error; err != nil {
		panic(err)
	}
	fmt.Printf("   超级管理员: admin / %s\n", password)

	fmt.Println("5. 清理媒体文件...")
	mediaDir := cfg.UploadDir
	if mediaDir == "" {
		mediaDir = "./storage/media"
	}
	absDir, err := filepath.Abs(mediaDir)
	if err == nil {
		if stat, err := os.Stat(absDir); err == nil && stat.IsDir() {
			entries, _ := os.ReadDir(absDir)
			for _, e := range entries {
				os.RemoveAll(filepath.Join(absDir, e.Name()))
			}
			fmt.Printf("   已清理 %s\n", absDir)
		} else {
			fmt.Printf("   目录不存在，跳过: %s\n", absDir)
		}
	} else {
		fmt.Printf("   获取目录路径失败: %v\n", err)
	}

	fmt.Println("\n数据库重置完成！")
}
