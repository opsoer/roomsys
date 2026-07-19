// Package handlers 提供后台定时检查任务，如自动更新到期合同状态
package handlers

import (
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// AutoCheckExpiringContracts 自动检查到期合同，创建退租待办任务
func AutoCheckExpiringContracts(db *gorm.DB) {
	now := utils.Now()
	expireThreshold := now.AddDate(0, 0, 30)

	var contracts []models.RentalContract
	db.Where("status = ? AND end_date != '' AND end_date <= ?",
		"active", expireThreshold.Format("2006-01-02")).
		Preload("Room").
		Limit(100).
		Find(&contracts)

	for _, contract := range contracts {
		endDate, err := time.Parse("2006-01-02", contract.EndDate)
		if err != nil {
			continue
		}

		isExpired := now.After(endDate)

		if isExpired {
			var existingTask models.Task
			result := db.Where("room_id = ? AND type = ? AND status = ?",
				contract.RoomID, "expired_room", "pending").First(&existingTask)
			if result.Error == gorm.ErrRecordNotFound {
				task := models.Task{
					BuildingID: contract.BuildingID,
					Title:      "房间到期退租",
					Type:       "expired_room",
					Status:     "pending",
					RoomID:     &contract.RoomID,
				}
				db.Create(&task)
				logger.Log.Info().
					Uint("room_id", contract.RoomID).
					Uint("building_id", contract.BuildingID).
					Msg("创建到期退租任务")
			}
		}
	}

	AutoCheckOverdueReservations(db)
}

// AutoCheckOverdueReservations 检查已交定金但到约定入住日仍未确认签约的预订，创建待办任务提醒房东。
// 注意：仅对“房间当前为空置（vacant）的预订”建任务；若房间仍在租（rented/expiring/expired），
// 说明该 reserved 合同只是“未来预订”，老租客尚未退租，不应误报为到入住日未签约。
func AutoCheckOverdueReservations(db *gorm.DB) {
	now := utils.Now()

	var contracts []models.RentalContract
	db.Where("status = ? AND start_date != '' AND start_date <= ?",
		"reserved", now.Format("2006-01-02")).
		Preload("Room").
		Limit(100).
		Find(&contracts)

	for _, contract := range contracts {
		// 房间仍在出租中，说明这是“未来预订”，跳过，不创建超时任务
		if contract.Room.ID != 0 && contract.Room.Status != "vacant" {
			continue
		}
		var existingTask models.Task
		result := db.Where("room_id = ? AND type = ? AND status = ?",
			contract.RoomID, "reserved_overdue", "pending").First(&existingTask)
		if result.Error == gorm.ErrRecordNotFound {
			task := models.Task{
				BuildingID:  contract.BuildingID,
				Title:       contract.Room.RoomNumber + " 定金预订已到入住日，请确认签约或处理",
				Type:        "reserved_overdue",
				Status:      "pending",
				RoomID:      &contract.RoomID,
				Description: "该房间已收取定金并到达约定入住日期，请及时确认签约或取消预订",
			}
			db.Create(&task)
			logger.Log.Info().
				Uint("room_id", contract.RoomID).
				Uint("building_id", contract.BuildingID).
				Msg("创建预订超时待办任务")
		}
	}
}
