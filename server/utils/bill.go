package utils

import (
	"fmt"
	"rental-server/models"
	"time"

	"gorm.io/gorm"
)

type FinanceRow struct {
	Type  string
	Total float64
}

func QueryMonthlyFinance(db *gorm.DB, buildingID uint, month string) MonthlyFinanceSummary {
	var rows []FinanceRow
	startDate := month + "-01"
	endDate := fmt.Sprintf("%s-01", NextMonth(month))
	db.Model(&models.Bill{}).
		Select("type, COALESCE(SUM(amount), 0) as total").
		Where("building_id = ? AND bill_date >= ? AND bill_date < ?", buildingID, startDate, endDate).
		Group("type").
		Find(&rows)

	var totalIncome, totalExpense float64
	for _, r := range rows {
		if r.Type == "income" {
			totalIncome = r.Total
		} else {
			totalExpense = r.Total
		}
	}
	return NewMonthlyFinanceSummary(totalIncome, totalExpense)
}

func NextMonth(month string) string {
	t, _ := time.Parse("2006-01", month)
	return t.AddDate(0, 1, 0).Format("2006-01")
}
