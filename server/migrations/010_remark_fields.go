package migrations

import (
	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// upRemarkFields 为业务表补「内部备注」列（2026-11）。
//
// 背景：很多关键动作（签约/预订/退租/账单/分红/待办/凭证…）只有结构化字段，
// 表格描述不清楚的信息（车牌号、扣款原因、特殊约定）无处可写，补一列自由文本。
//
// 可见性约定（重要）：除 Public=false 的少数几张表外，备注一律**仅后台可见**。
// 会出现在公开接口里的模型（buildings / building_landlords / rooms / room_media）
// 在模型层就是 json:"-"，只能通过后台 DTO 透出，详见 models 包注释。
// location_customs 的公开接口用的是显式最小字段 DTO，备注不会泄露。
func upRemarkFields(db *gorm.DB) error {
	fields := []struct {
		model interface{}
		field string
		label string
	}{
		{&models.RentalContract{}, "Remark", "rental_contracts.remark"},
		{&models.Tenant{}, "Remark", "tenants.remark"},
		{&models.Bill{}, "Remark", "bills.remark"},
		{&models.BillEvidence{}, "Remark", "bill_evidences.remark"},
		{&models.BuildingLandlord{}, "Remark", "building_landlords.remark"},
		{&models.Shareholder{}, "Remark", "shareholders.remark"},
		{&models.Dividend{}, "Remark", "dividends.remark"},
		{&models.Task{}, "ResultRemark", "tasks.result_remark"},
		{&models.RoomMedia{}, "Remark", "room_media.remark"},
		{&models.Building{}, "Remark", "buildings.remark"},
		{&models.Room{}, "Remark", "rooms.remark"},
		{&models.User{}, "Remark", "users.remark"},
		{&models.LocationCustom{}, "Remark", "location_customs.remark"},
		{&models.RecruitSubmission{}, "Remark", "recruit_submissions.remark"},
		{&models.BuildingRenewal{}, "Remark", "building_renewals.remark"},
	}
	for _, f := range fields {
		if !db.Migrator().HasTable(f.model) {
			continue
		}
		if db.Migrator().HasColumn(f.model, f.field) {
			continue
		}
		if err := db.Migrator().AddColumn(f.model, f.field); err != nil {
			return err
		}
		logger.Log.Info().Str("column", f.label).Msg("迁移 010：已添加备注列")
	}
	logger.Log.Info().Msg("迁移 010：备注字段就绪")
	return nil
}

// downRemarkFields 回退：删除本次新增的备注列（数据不可恢复）。
func downRemarkFields(db *gorm.DB) error {
	fields := []struct {
		model interface{}
		field string
	}{
		{&models.RentalContract{}, "Remark"},
		{&models.Tenant{}, "Remark"},
		{&models.Bill{}, "Remark"},
		{&models.BillEvidence{}, "Remark"},
		{&models.BuildingLandlord{}, "Remark"},
		{&models.Shareholder{}, "Remark"},
		{&models.Dividend{}, "Remark"},
		{&models.Task{}, "ResultRemark"},
		{&models.RoomMedia{}, "Remark"},
		{&models.Building{}, "Remark"},
		{&models.Room{}, "Remark"},
		{&models.User{}, "Remark"},
		{&models.LocationCustom{}, "Remark"},
		{&models.RecruitSubmission{}, "Remark"},
		{&models.BuildingRenewal{}, "Remark"},
	}
	for _, f := range fields {
		if !db.Migrator().HasTable(f.model) {
			continue
		}
		if !db.Migrator().HasColumn(f.model, f.field) {
			continue
		}
		if err := db.Migrator().DropColumn(f.model, f.field); err != nil {
			return err
		}
	}
	return nil
}
