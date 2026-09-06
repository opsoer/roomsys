// 工具包，提供账单财务相关查询功能
package utils

import (
	"fmt"
	"rental-server/models"
	"strconv"
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

// NextBillNo 生成下一个账单编号。
// 统计范围包含软删除记录：bill_no 唯一索引仍被软删除行占用，
// 若按未删除行计数会生成冲突编号导致当天后续账单全部创建失败。
// 并发冲突由 bills.bill_no 唯一索引兜底，调用方事务失败后整体回滚重试。
func NextBillNo(db *gorm.DB, datePart string) string {
	var lastNo string
	db.Unscoped().Model(&models.Bill{}).
		Where("bill_no LIKE ?", "B"+datePart+"%").
		Select("bill_no").
		Order("bill_no DESC").
		Limit(1).
		Scan(&lastNo)

	next := 1
	prefixLen := 1 + len(datePart) // "B" + 8位日期
	if len(lastNo) > prefixLen {
		if n, err := strconv.Atoi(lastNo[prefixLen:]); err == nil && n >= next {
			next = n + 1
		}
	}
	return fmt.Sprintf("B%s%05d", datePart, next)
}
