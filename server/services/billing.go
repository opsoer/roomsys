// 房间服务：账期计算与账单生成。
//
// 这些函数原先位于 handlers 包内，导致「计费口径」只有 HTTP 层能调用、定时任务
// 与出租流程各写一份。下沉到 services 后：出租流程、月度账单定时任务、导入流程
// 共用同一套口径。
package services

import (
	"errors"
	"fmt"
	"time"

	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// RentBillsParams 出租建账参数。
type RentBillsParams struct {
	// RentPrice 月租金。
	RentPrice float64
	// ManagementFee 月管理费。
	ManagementFee float64
	// Deposit 押金金额。
	Deposit float64
	// StartDate 起租日（YYYY-MM-DD）。
	StartDate string
	// EndDate 到期日（YYYY-MM-DD）。
	EndDate string
	// RecordDepositBill 是否记录押金账单；nil 时按「起租日不早于本月1日」自动判断，
	// 避免补记历史租约时重复记押金。
	RecordDepositBill *bool
}

// CalcMonthDays 计算给定月份的天数。
func CalcMonthDays(t time.Time) int {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

// BillPeriod 计算合同在指定月份内的计费区间：账期 = 租期与该月的交集。
// 供出租补记与月度租金定时任务共用，必须保持同一套收敛口径。
func BillPeriod(contractStart, contractEnd, monthStart, monthEnd time.Time) (billStart, billEnd time.Time) {
	billStart, billEnd = monthStart, monthEnd
	if contractStart.After(monthStart) {
		billStart = contractStart
	}
	if contractEnd.Before(monthEnd) {
		billEnd = contractEnd
	}
	return billStart, billEnd
}

// RentAndMgmtAmounts 按统一口径计算租金与管理费金额：
//   - 整月：直接向零截断到分（int 转换，不四舍五入），仅做浮点残差清理；
//   - 非整月：按天折算，走 utils.CalcProratedAmount（按分四舍五入）。
//     注意两条路径的修约方式不同（截断 vs 四舍五入），保持现状勿混用。
func RentAndMgmtAmounts(rentPrice, mgmtFee float64, fullMonth bool, billStart, billEnd time.Time, daysInMonth int) (rentAmount, mgmtAmount float64) {
	if fullMonth {
		return float64(int(rentPrice*100)) / 100, float64(int(mgmtFee*100)) / 100
	}
	return utils.CalcProratedAmount(rentPrice, billStart, billEnd, daysInMonth),
		utils.CalcProratedAmount(mgmtFee, billStart, billEnd, daysInMonth)
}

// CreateRentedBills 生成出租相关账单（房间租金账单 + 押金账单）。
//   - 租金账单：按账期内的租金+管理费生成，描述含「租金X元，管理费Y元（起止日）」
//   - 押金账单：可选。RecordDepositBill 为 nil 时按「起租日不早于本月1日」自动判断，
//     避免补记历史租约时重复记押金；押金为 0 时不记录。
func CreateRentedBills(tx *gorm.DB, buildingID, roomID, uid uint, p RentBillsParams) error {
	now := utils.Now()
	datePart := now.Format("20060102")
	monthStart := utils.FirstDayOfMonth(now)
	monthEnd := utils.LastDayOfMonth(now)

	startDate, err1 := time.Parse("2006-01-02", p.StartDate)
	endDate, err2 := time.Parse("2006-01-02", p.EndDate)
	if err1 != nil || err2 != nil {
		return errors.New("起止日期格式错误")
	}

	// 账期 = 租期 ∩ 当前自然月（BillPeriod 以月首/月末兜底，向内收缩——
	//   billStart 只会晚于或等于月首，billEnd 只会早于或等于月末）：
	//   - 起租日在本月内：billStart = 起租日，非整月按 (billEnd-billStart+1)/本月天数
	//     折算（CalcProratedAmount 两端日均计入）；
	//   - 补记历史租约（起租日早于本月）：billStart = 月首，与整月租约一样按月计费
	//     （到期日不早于本月末时 fullMonth 为 true 走整月价，否则按期内天数折算）。
	//   两种情形折算天数都不会超出"本月天数"分母，金额不超过一整月；
	//   单次建账只按当前月口径，历史月份不逐月补账。
	billStart, billEnd := BillPeriod(startDate, endDate, monthStart, monthEnd)
	daysInMonth := monthEnd.Day()

	if p.RentPrice > 0 && !billStart.After(billEnd) {
		fullMonth := billStart.Equal(monthStart) && billEnd.Equal(monthEnd)
		rentAmount, mgmtAmount := RentAndMgmtAmounts(p.RentPrice, p.ManagementFee, fullMonth, billStart, billEnd, daysInMonth)
		totalAmount := float64(int((rentAmount+mgmtAmount)*100)) / 100

		if totalAmount > 0 {
			rentDesc := billStart.Format("2006-01-02") + " ~ " + billEnd.Format("2006-01-02")
			rentBill := models.Bill{
				BillNo:      NextBillNo(tx, datePart),
				Type:        "income",
				Subtype:     "租金",
				Amount:      totalAmount,
				BuildingID:  buildingID,
				RoomID:      &roomID,
				Description: fmt.Sprintf("租金%.2f元，管理费%.2f元（%s）", rentAmount, mgmtAmount, rentDesc),
				BillDate:    now.Format("2006-01-02"),
				PaidStatus:  "paid",
				CreatedBy:   uid,
			}
			if err := tx.Create(&rentBill).Error; err != nil {
				return err
			}
		}
	}

	recordDeposit := !startDate.Before(monthStart) // nil 默认：起租日不早于本月1日（含未来起租）才记押金
	if p.RecordDepositBill != nil {
		recordDeposit = *p.RecordDepositBill
	}
	if p.Deposit > 0 && recordDeposit {
		depositBill := models.Bill{
			BillNo:      NextBillNo(tx, datePart),
			Type:        "income",
			Subtype:     "押金",
			Amount:      p.Deposit,
			BuildingID:  buildingID,
			RoomID:      &roomID,
			Description: "押金：签约押金",
			BillDate:    now.Format("2006-01-02"),
			PaidStatus:  "paid",
			CreatedBy:   uid,
		}
		if err := tx.Create(&depositBill).Error; err != nil {
			return err
		}
	}
	return nil
}

// CreateDepositRefundBill 生成押金退还账单（退租用）。
// refunded <= 0 时不生成账单（负数同样跳过）。
func CreateDepositRefundBill(tx *gorm.DB, buildingID, roomID, uid uint, originalDeposit, refunded float64) error {
	if refunded <= 0 {
		return nil
	}
	now := utils.Now()
	deducted := originalDeposit - refunded
	desc := fmt.Sprintf("押金退还：原押金%.2f元，已退款%.2f元", originalDeposit, refunded)
	if deducted > 0 {
		desc += fmt.Sprintf("（扣除%.2f元）", deducted)
	}
	bill := models.Bill{
		BillNo:      NextBillNo(tx, now.Format("20060102")),
		Type:        "expense",
		Subtype:     "押金退还",
		Amount:      refunded,
		BuildingID:  buildingID,
		RoomID:      &roomID,
		Description: desc,
		BillDate:    now.Format("2006-01-02"),
		PaidStatus:  "paid",
		CreatedBy:   uid,
	}
	return tx.Create(&bill).Error
}

// CreateEarnestBreachBill 生成定金违约账单（取消预订用）。
// diff = 原收定金 - 实际退还：正数表示扣留定金（收入），负数表示房东多退（支出）。
func CreateEarnestBreachBill(tx *gorm.DB, buildingID, roomID, uid uint, earnest, refunded float64) error {
	diff := float64(int((earnest-refunded)*100)) / 100
	if diff == 0 {
		return nil
	}
	now := utils.Now()
	bill := models.Bill{
		BillNo:      NextBillNo(tx, now.Format("20060102")),
		BuildingID:  buildingID,
		RoomID:      &roomID,
		Subtype:     "定金违约",
		BillDate:    now.Format("2006-01-02"),
		PaidStatus:  "paid",
		CreatedBy:   uid,
		Description: fmt.Sprintf("取消预订：原收定金%.2f元，实际退还%.2f元", earnest, refunded),
	}
	if diff > 0 {
		bill.Type = "income"
		bill.Amount = diff
		bill.Description += fmt.Sprintf("，租客违约/不租，扣留定金%.2f元", diff)
	} else {
		bill.Type = "expense"
		bill.Amount = -diff
		bill.Description += fmt.Sprintf("，房东违约/主动多退，额外赔付%.2f元", -diff)
	}
	return tx.Create(&bill).Error
}
