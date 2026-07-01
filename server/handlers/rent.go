package handlers

import (
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

func AutoCreateMonthlyRentBills(db *gorm.DB) {
	now := utils.Now()
	month := now.Format("2006-01")

	var contracts []models.RentalContract
	db.Where("status = ?", "active").Find(&contracts)

	for _, contract := range contracts {
		var existingBill models.Bill
		result := db.Where("room_id = ? AND subtype = ? AND bill_date LIKE ?",
			contract.RoomID, "租金", month+"%").First(&existingBill)
		if result.Error == nil {
			continue
		}

		daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
		dailyRate := contract.RentPrice / float64(daysInMonth)
		amount := dailyRate * float64(now.Day())
		amount = float64(int(amount*100)) / 100

		bill := models.Bill{
			BuildingID:  contract.BuildingID,
			BillNo:      utils.GenerateBillNo(),
			Type:        "income",
			Subtype:     "租金",
			Amount:      amount,
			RoomID:      &contract.RoomID,
			Description: "当月租金",
			BillDate:    now.Format("2006-01-02"),
		}
		if err := db.Create(&bill).Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", contract.RoomID).Msg("创建月度租金账单失败")
		}
	}
}
