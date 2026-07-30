// Package handlers 提供后台定时检查任务，如自动更新到期合同状态
package handlers

import (
	"fmt"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// AutoCheckExpiringContracts 自动检查到期合同，创建退租待办任务
func AutoCheckExpiringContracts(db *gorm.DB) string {
	now := utils.Now()
	expireThreshold := now.AddDate(0, 0, 30)

	logger.Log.Info().Str("threshold", expireThreshold.Format("2006-01-02")).Msg("AutoCheckExpiringContracts: 开始执行")

	var contracts []models.RentalContract
	db.Where("status = ? AND end_date != '' AND end_date <= ?",
		"active", expireThreshold.Format("2006-01-02")).
		Preload("Room").
		Limit(100).
		Find(&contracts)

	taskCount := 0
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
				taskCount++
				logger.Log.Info().
					Uint("room_id", contract.RoomID).
					Uint("building_id", contract.BuildingID).
					Msg("创建到期退租任务")
			}
		}
	}

	result := fmt.Sprintf("合同检查: 创建 %d 条退租待办", taskCount)
	logger.Log.Info().Int("task_count", taskCount).Msg("AutoCheckExpiringContracts: 执行完毕")

	overdueResult := AutoCheckOverdueReservations(db)
	return result + "，" + overdueResult
}

// CheckExpiredBuildings 检查所有到期公寓，将已过期的状态更新为 expired。
func CheckExpiredBuildings(db *gorm.DB) string {
	var buildings []models.Building
	db.Where("status = ? AND expired_at IS NOT NULL AND expired_at != ''", "active").Find(&buildings)
	now := utils.Now()
	expiredCount := 0
	for _, b := range buildings {
		if expDate, err := time.Parse("2006-01-02", b.ExpiredAt); err == nil {
			if now.After(expDate) {
				db.Model(&b).Update("status", "expired")
				expiredCount++
				logger.Log.Info().
					Uint("building_id", b.ID).
					Str("name", b.Name).
					Str("expired_at", b.ExpiredAt).
					Msg("公寓已到期，状态更新为 expired")
			}
		}
	}
	result := fmt.Sprintf("公寓到期: %d 栋已到期", expiredCount)
	logger.Log.Info().Int("count", expiredCount).Msg("到期公寓检查完成")
	return result
}

// AutoCleanupData 清理超过90天的软删除数据和 page_views 记录。
func AutoCleanupData(db *gorm.DB) string {
	cleanupResult := ""
	if err := models.CleanupSoftDeleted(db, 90); err != nil {
		logger.Log.Error().Err(err).Msg("软删除数据清理失败")
		cleanupResult = "软删除: 失败"
	} else {
		logger.Log.Info().Msg("软删除数据清理完成")
		cleanupResult = "软删除: 完成"
	}
	cutoff := time.Now().AddDate(0, 0, -90)
	pvResult := db.Where("created_at < ?", cutoff).Delete(&models.PageView{}).Error
	if pvResult != nil {
		logger.Log.Error().Err(pvResult).Msg("page_views 清理失败")
		cleanupResult += "，PageView: 失败"
	} else {
		logger.Log.Info().Msg("page_views 清理完成")
		cleanupResult += "，PageView: 完成"
	}
	return "数据清理: " + cleanupResult
}

// AutoCheckOverdueReservations 检查已交定金但到约定入住日仍未确认签约的预订，创建待办任务提醒房东。
func AutoCheckOverdueReservations(db *gorm.DB) string {
	now := utils.Now()

	logger.Log.Info().Msg("AutoCheckOverdueReservations: 开始执行")

	var contracts []models.RentalContract
	db.Where("status = ? AND start_date != '' AND start_date <= ?",
		"reserved", now.Format("2006-01-02")).
		Preload("Room").
		Limit(100).
		Find(&contracts)

	taskCount := 0
	for _, contract := range contracts {
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
			taskCount++
			logger.Log.Info().
				Uint("room_id", contract.RoomID).
				Uint("building_id", contract.BuildingID).
				Msg("创建预订超时待办任务")
		}
	}

	result := fmt.Sprintf("预订超时: 创建 %d 条待办", taskCount)
	logger.Log.Info().Int("task_count", taskCount).Msg("AutoCheckOverdueReservations: 执行完毕")
	return result
}
