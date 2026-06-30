package utils

import (
	"rental-server/models"

	"gorm.io/gorm"
)

type FinanceRow struct {
	Type  string
	Total float64
}

func QueryMonthlyFinance(db *gorm.DB, buildingID uint, month string) MonthlyFinanceSummary {
	var rows []FinanceRow
	db.Model(&models.Bill{}).
		Select("type, COALESCE(SUM(amount), 0) as total").
		Where("building_id = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", buildingID, month).
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
