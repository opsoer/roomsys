// Package handlers 提供后台定时检查任务，如自动更新到期合同状态
package handlers

import (
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// AutoCheckExpiringContracts 自动检查到期合同，更新房间状态并创建退租任务
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

		newStatus := "rented"
		if now.After(endDate) {
			newStatus = "expired"
		} else if expireThreshold.After(endDate) {
			newStatus = "expiring"
		}

		if newStatus != contract.Room.Status {
			db.Model(&contract.Room).Update("status", newStatus)
			logger.Log.Info().
				Uint("room_id", contract.Room.ID).
				Str("old_status", contract.Room.Status).
				Str("new_status", newStatus).
				Msg("自动更新房间状态")
		}

		if newStatus == "expired" {
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
}
