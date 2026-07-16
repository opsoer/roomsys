// 工具包，提供账单财务相关查询功能
package utils

import (
	"fmt"
	"rental-server/models"
	"time"

	"gorm.io/gorm"
)

// FinanceRow 财务查询结果行，包含收支类型和金额
type FinanceRow struct {
	Type  string
	Total float64
}

// QueryMonthlyFinance 查询指定楼宇某月的收支汇总
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

// NextMonth 计算给定月份的下一个月，格式 "2006-01"
func NextMonth(month string) string {
	t, _ := time.Parse("2006-01", month)
	return t.AddDate(0, 1, 0).Format("2006-01")
}
