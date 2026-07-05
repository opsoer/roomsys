package handlers

import (
	"fmt"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

func calcMonthDays(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

func AutoCreateMonthlyRentBills(db *gorm.DB) {
	now := utils.Now()
	month := now.Format("2006-01")
	startDate := month + "-01"
	endDate := now.AddDate(0, 1, 0).Format("2006-01-01")

	var contracts []models.RentalContract
	db.Joins("LEFT JOIN bills ON bills.room_id = rental_contracts.room_id AND bills.subtype = '租金' AND bills.bill_date >= ? AND bills.bill_date < ?",
		startDate, endDate).
		Where("rental_contracts.status = ? AND bills.id IS NULL", "active").
		Find(&contracts)

	for _, contract := range contracts {
		contractStart, _ := time.Parse("2006-01-02", contract.StartDate)
		contractEnd, _ := time.Parse("2006-01-02", contract.EndDate)
		monthStart, _ := time.Parse("2006-01-02", startDate)
		monthEnd := monthStart.AddDate(0, 1, -1)

		billStart := monthStart
		billEnd := monthEnd
		daysInMonth := calcMonthDays(monthStart)

		var amount float64
		var descRange string

		if contractStart.After(monthStart) {
			billStart = contractStart
		}
		if contractEnd.Before(monthEnd) {
			billEnd = contractEnd
		}

		descRange = billStart.Format("2006-01-02") + " ~ " + billEnd.Format("2006-01-02")

		if billStart.Equal(monthStart) && billEnd.Equal(monthEnd) {
			amount = float64(int(contract.RentPrice*100)) / 100
		} else {
			amount = utils.CalcProratedAmount(contract.RentPrice, billStart, billEnd, daysInMonth)
		}

		if amount <= 0 {
			continue
		}

		datePart := now.Format("20060102")
		var count int64
		db.Model(&models.Bill{}).
			Where("building_id = ? AND bill_no LIKE ?", contract.BuildingID, "B"+datePart+"%").
			Count(&count)
		billNo := fmt.Sprintf("B%s%05d", datePart, count+1)

		bill := models.Bill{
			BuildingID:  contract.BuildingID,
			BillNo:      billNo,
			Type:        "income",
			Subtype:     "租金",
			Amount:      amount,
			RoomID:      &contract.RoomID,
			Description: "租金：" + descRange,
			BillDate:    now.Format("2006-01-02"),
		}
		if err := db.Create(&bill).Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", contract.RoomID).Msg("创建月度租金账单失败")
		}
	}
}
