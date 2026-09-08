// Package handlers 自动创建月度租金账单，处理按比例分摊计算
package handlers

import (
	"fmt"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// calcMonthDays 计算给定月份的天数
func calcMonthDays(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// billPeriod 计算合同在指定月份内的计费区间：账期 = 租期与该月的交集。
// 供出租补记与月度租金定时任务共用，必须保持同一套收敛口径。
func billPeriod(contractStart, contractEnd, monthStart, monthEnd time.Time) (billStart, billEnd time.Time) {
	billStart, billEnd = monthStart, monthEnd
	if contractStart.After(monthStart) {
		billStart = contractStart
	}
	if contractEnd.Before(monthEnd) {
		billEnd = contractEnd
	}
	return billStart, billEnd
}

// rentAndMgmtAmounts 按统一口径计算租金与管理费金额：整月直接取整到分，非整月按天折算
func rentAndMgmtAmounts(rentPrice, mgmtFee float64, fullMonth bool, billStart, billEnd time.Time, daysInMonth int) (rentAmount, mgmtAmount float64) {
	if fullMonth {
		return float64(int(rentPrice*100)) / 100, float64(int(mgmtFee*100)) / 100
	}
	return utils.CalcProratedAmount(rentPrice, billStart, billEnd, daysInMonth),
		utils.CalcProratedAmount(mgmtFee, billStart, billEnd, daysInMonth)
}

// AutoCreateMonthlyRentBills 自动生成当月所有活跃合同的租金账单
func AutoCreateMonthlyRentBills(db *gorm.DB) string {
	now := utils.Now()
	month := now.Format("2006-01")
	startDate := month + "-01"
	endDate := now.AddDate(0, 1, 0).Format("2006-01-02")

	logger.Log.Info().
		Str("now", now.Format(time.RFC3339)).
		Str("month", month).
		Str("startDate", startDate).
		Str("endDate", endDate).
		Msg("AutoCreateMonthlyRentBills: 开始执行")

	var contracts []models.RentalContract
	db.Joins("LEFT JOIN bills ON bills.room_id = rental_contracts.room_id AND bills.subtype = '租金' AND bills.bill_date >= ? AND bills.bill_date < ?",
		startDate, endDate).
		Where("rental_contracts.status = ? AND bills.id IS NULL", "active").
		Find(&contracts)

	logger.Log.Info().
		Int("contracts_found", len(contracts)).
		Msg("AutoCreateMonthlyRentBills: 查询到待创建账单的合同数")

	billsCreated := 0
	monthStart := utils.FirstDayOfMonth(now)
	monthEnd := utils.LastDayOfMonth(now)
	daysInMonth := calcMonthDays(monthStart)
	for _, contract := range contracts {
		if contract.EndDate != "" {
			if contractEnd, err := time.Parse("2006-01-02", contract.EndDate); err == nil && contractEnd.Before(monthStart) {
				continue // 租约在本月之前已结束，本月无账
			}
		}

		contractStart, _ := time.Parse("2006-01-02", contract.StartDate)
		contractEnd, _ := time.Parse("2006-01-02", contract.EndDate)

		billStart, billEnd := billPeriod(contractStart, contractEnd, monthStart, monthEnd)
		descRange := billStart.Format("2006-01-02") + " ~ " + billEnd.Format("2006-01-02")

		var roomManagementFee float64
		var room models.Room
		if err := db.First(&room, contract.RoomID).Error; err == nil {
			if room.ManagementFee != nil {
				roomManagementFee = *room.ManagementFee
			}
		}

		fullMonth := billStart.Equal(monthStart) && billEnd.Equal(monthEnd)
		rentAmount, mgmtAmount := rentAndMgmtAmounts(contract.RentPrice, roomManagementFee, fullMonth, billStart, billEnd, daysInMonth)
		if rentAmount <= 0 {
			continue
		}

		totalAmount := float64(int((rentAmount+mgmtAmount)*100)) / 100

		billNo := utils.NextBillNo(db, now.Format("20060102"))

		bill := models.Bill{
			BuildingID:  contract.BuildingID,
			BillNo:      billNo,
			Type:        "income",
			Subtype:     "租金",
			Amount:      totalAmount,
			RoomID:      &contract.RoomID,
			Description: fmt.Sprintf("租金：%.2f元，管理费：%.2f元（%s）", rentAmount, mgmtAmount, descRange),
			BillDate:    now.Format("2006-01-02"),
		}
		if err := db.Create(&bill).Error; err != nil {
			logger.Log.Error().Err(err).Uint("room_id", contract.RoomID).Msg("创建月度租金账单失败")
		} else {
			billsCreated++
			logger.Log.Info().
				Uint("room_id", contract.RoomID).
				Str("bill_no", bill.BillNo).
				Float64("amount", bill.Amount).
				Str("bill_date", bill.BillDate).
				Msg("AutoCreateMonthlyRentBills: 账单创建成功")
		}
	}

	result := fmt.Sprintf("租金账单: 创建 %d 条", billsCreated)
	logger.Log.Info().Int("bills_created", billsCreated).Msg("AutoCreateMonthlyRentBills: 执行完毕")
	return result
}
