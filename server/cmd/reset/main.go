package main

import (
	"fmt"
	"os"
	"path/filepath"

	"rental-server/config"
	"rental-server/database"
	"rental-server/models"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg)
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
		&models.BillEvidence{},
		&models.RentalContract{},
		&models.Tenant{},
		&models.RoomMedia{},
		&models.Room{},
		&models.BuildingLandlord{},
		&models.BuildingRenewal{},
		&models.Building{},
		&models.User{},
		&models.StatTotal{},
		&models.StatIP{},
		&models.StatDailyIP{},
		&models.StatDaily{},
		&models.LocationCustom{},
		&models.LocationRename{},
	)
	fmt.Println("   完成")

	fmt.Println("2. 重建表结构...")
	if err := models.AutoMigrate(db); err != nil {
		panic(err)
	}
	fmt.Println("   完成")

	fmt.Println("3. 清空数据面板统计(page_views 与聚合表)数据...")
	for _, table := range []interface{}{
		&models.PageView{},
		&models.StatTotal{},
		&models.StatIP{},
		&models.StatDailyIP{},
		&models.StatDaily{},
	} {
		if err := db.Where("1 = 1").Delete(table).Error; err != nil {
			panic(err)
		}
	}
	fmt.Println("   完成")

	fmt.Println("4. 创建超级管理员（来自 config.yaml 的 admin 段）...")
	password := cfg.Admin.Password
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	admin := models.User{
		Username:     cfg.Admin.Username,
		PasswordHash: string(hash),
		Role:         "super_admin",
	}
	if err := db.Create(&admin).Error; err != nil {
		panic(err)
	}
	fmt.Printf("   超级管理员: %s / %s\n", cfg.Admin.Username, password)

	fmt.Println("5. 清理媒体文件...")
	mediaDir := cfg.Server.UploadDir
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
