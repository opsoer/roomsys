package main

import (
	"fmt"
	"rental-server/config"
	"rental-server/models"
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

	db.Migrator().DropTable(
		&models.User{},
		&models.Building{},
		&models.BuildingLandlord{},
		&models.Room{},
		&models.RoomMedia{},
		&models.Tenant{},
		&models.RentalContract{},
		&models.Bill{},
		&models.Shareholder{},
		&models.Dividend{},
		&models.Task{},
	)
	fmt.Println("All tables dropped successfully")
}
