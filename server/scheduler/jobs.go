// Package scheduler 提供后台定时检查任务，如自动更新到期合同状态
package scheduler

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
	expireThreshold := now.AddDate(0, 0, utils.ExpiringSoonDays)

	logger.Log.Debug().Str("threshold", expireThreshold.Format("2006-01-02")).Msg("AutoCheckExpiringContracts: 开始执行")

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
				bid := contract.BuildingID
				task := models.Task{
					BuildingID: &bid,
					Title:      "房间到期退租",
					Type:       "expired_room",
					Status:     "pending",
					RoomID:     &contract.RoomID,
				}
				db.Create(&task)
				taskCount++
				logger.Log.Debug().
					Uint("room_id", contract.RoomID).
					Uint("building_id", contract.BuildingID).
					Msg("创建到期退租任务")
			}
		}
	}

	result := fmt.Sprintf("合同检查: 创建 %d 条退租待办", taskCount)
	logger.Log.Debug().Int("task_count", taskCount).Msg("AutoCheckExpiringContracts: 执行完毕")

	overdueResult := AutoCheckOverdueReservations(db)
	return result + "，" + overdueResult
}

// CheckExpiredBuildings 检查所有公寓的到期情况（每天凌晨3点定时执行）：
//   - 即将到期（10天内）：状态更新为 expiring，并为房东和超级管理员创建续约提醒待办
//   - 已到期：状态更新为 hidden（自动不可见），并为房东和超级管理员创建到期待办
func CheckExpiredBuildings(db *gorm.DB) string {
	now := utils.Now()
	expireThreshold := now.AddDate(0, 0, utils.ExpiringSoonDays)

	logger.Log.Debug().Str("threshold", expireThreshold.Format("2006-01-02")).Msg("CheckExpiredBuildings: 开始执行")

	var buildings []models.Building
	db.Where("status IN ? AND expired_at IS NOT NULL AND expired_at != ''", []string{"active", "expiring"}).Find(&buildings)

	expiringCount := 0
	expiredCount := 0
	statusChanged := false
	for _, b := range buildings {
		expDate, err := time.Parse("2006-01-02", b.ExpiredAt)
		if err != nil {
			continue
		}
		// 防止同一公寓在已到期后又重复创建"即将到期"待办
		if now.After(expDate) {
			if b.Status != "hidden" {
				db.Model(&b).Update("status", "hidden")
				statusChanged = true
				logger.Log.Info().
					Uint("building_id", b.ID).
					Str("name", b.Name).
					Str("expired_at", b.ExpiredAt).
					Msg("公寓已到期，状态更新为 hidden（不可见）")
			}
			createBuildingExpiryTasks(db, b, "building_expired",
				"公寓「"+b.Name+"」已到期，请尽快续约",
				"公寓已于 "+b.ExpiredAt+" 到期，系统已自动将其设为不可见状态，请及时续约")
			expiredCount++
		} else if !expDate.After(expireThreshold) {
			if b.Status != "expiring" {
				db.Model(&b).Update("status", "expiring")
				statusChanged = true
				logger.Log.Info().
					Uint("building_id", b.ID).
					Str("name", b.Name).
					Str("expired_at", b.ExpiredAt).
					Msg("公寓即将到期，状态更新为 expiring")
			}
			createBuildingExpiryTasks(db, b, "building_expiring",
				"公寓「"+b.Name+"」即将到期，请及时续约",
				"公寓将于 "+b.ExpiredAt+" 到期，请尽快续约以免到期后自动隐藏")
			expiringCount++
		}
	}
	if statusChanged {
		// 状态影响价格参考等按 active 过滤的统计，立即失效缓存
		utils.ClearStatsCache()
	}

	result := fmt.Sprintf("公寓到期: %d 栋已到期, %d 栋即将到期", expiredCount, expiringCount)
	logger.Log.Debug().Int("expired", expiredCount).Int("expiring", expiringCount).Msg("到期公寓检查完成")
	return result
}

// createBuildingExpiryTasks 为公寓到期/即将到期创建待办（房东后台 + 超级管理员各一条，幂等）
func createBuildingExpiryTasks(db *gorm.DB, b models.Building, taskType, title, description string) {
	bid := b.ID
	for _, scope := range []string{"building", "platform"} {
		var existing models.Task
		result := db.Where("building_id = ? AND type = ? AND scope = ? AND status = ?",
			b.ID, taskType, scope, "pending").First(&existing)
		if result.Error != gorm.ErrRecordNotFound {
			continue
		}
		task := models.Task{
			BuildingID:  &bid,
			Title:       title,
			Type:        taskType,
			Priority:    "high",
			Status:      "pending",
			Scope:       scope,
			DueDate:     b.ExpiredAt,
			Description: description,
		}
		db.Create(&task)
		logger.Log.Debug().
			Uint("building_id", b.ID).
			Str("type", taskType).
			Str("scope", scope).
			Msg("创建公寓到期提醒待办")
	}
}

// AutoCleanupData 清理超过90天的软删除数据、page_views 记录与死楼栋孤儿统计行，并修剪每日去重 IP 表。
func AutoCleanupData(db *gorm.DB) string {
	cleanupResult := ""
	if err := models.CleanupSoftDeleted(db, 90); err != nil {
		logger.Log.Error().Err(err).Msg("软删除数据清理失败")
		cleanupResult = "软删除: 失败"
	} else {
		logger.Log.Debug().Msg("软删除数据清理完成")
		cleanupResult = "软删除: 完成"
	}
	cutoff := time.Now().AddDate(0, 0, -90)
	pvResult := db.Where("created_at < ?", cutoff).Delete(&models.PageView{}).Error
	if pvResult != nil {
		logger.Log.Error().Err(pvResult).Msg("page_views 清理失败")
		cleanupResult += "，PageView: 失败"
	} else {
		logger.Log.Debug().Msg("page_views 清理完成")
		cleanupResult += "，PageView: 完成"
	}
	// stat_daily_ips 每日去重 IP 保留 370 天：覆盖趋势接口最大 365 天窗口，
	// 更早的日 UV 已无从展示，修剪避免表无限膨胀（平台总 UV 在 stat_ips，不受影响）
	dayIPCutoff := time.Now().AddDate(0, 0, -370).Format("2006-01-02")
	if err := db.Where("day < ?", dayIPCutoff).Delete(&models.StatDailyIP{}).Error; err != nil {
		logger.Log.Warn().Err(err).Msg("每日去重 IP 清理失败")
	}
	// stat_daily 孤儿楼栋行兜底清理：楼栋已被硬删/软删（如绕过 BuildingService.Delete
	// 的路径、迁移回填、异步竞争留下的行）时清除；排行查询已按存活楼栋过滤，
	// 此处主要防存储无限增长。平台行（building_id=0）与存活楼栋不受影响。
	if err := db.Exec(
		"DELETE FROM stat_daily WHERE building_id > 0 AND building_id NOT IN " +
			"(SELECT id FROM buildings WHERE deleted_at IS NULL)").Error; err != nil {
		logger.Log.Warn().Err(err).Msg("stat_daily 孤儿楼栋行清理失败")
	}
	// page_views 孤儿楼栋行兜底清理：与 stat_daily 同源（异步竞争、历史软删），
	// 平台 PV/UV 已落在聚合表，删除这些明细不影响面板总量；存活楼栋与 building_id=0
	// 的列表页明细不受影响。
	if err := db.Exec(
		"DELETE FROM page_views WHERE building_id > 0 AND building_id NOT IN " +
			"(SELECT id FROM buildings WHERE deleted_at IS NULL)").Error; err != nil {
		logger.Log.Warn().Err(err).Msg("page_views 孤儿楼栋行清理失败")
	}
	// 审计日志保留 180 天，避免 audit_logs 表无限膨胀
	auditCutoff := time.Now().AddDate(0, 0, -180)
	if err := db.Where("created_at < ?", auditCutoff).Delete(&models.AuditLog{}).Error; err != nil {
		logger.Log.Warn().Err(err).Msg("审计日志清理失败")
	}
	// 清理可能改动了 page_views/stat_daily，立即失效统计缓存
	utils.ClearStatsCache()
	return "数据清理: " + cleanupResult
}

// AutoCheckOverdueReservations 检查已交定金但到约定入住日仍未确认签约的预订，创建待办任务提醒房东。
func AutoCheckOverdueReservations(db *gorm.DB) string {
	now := utils.Now()

	logger.Log.Debug().Msg("AutoCheckOverdueReservations: 开始执行")

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
			bid := contract.BuildingID
			task := models.Task{
				BuildingID:  &bid,
				Title:       contract.Room.RoomNumber + " 定金预订已到入住日，请确认签约或处理",
				Type:        "reserved_overdue",
				Status:      "pending",
				RoomID:      &contract.RoomID,
				Description: "该房间已收取定金并到达约定入住日期，请及时确认签约或取消预订",
			}
			db.Create(&task)
			taskCount++
			logger.Log.Debug().
				Uint("room_id", contract.RoomID).
				Uint("building_id", contract.BuildingID).
				Msg("创建预订超时待办任务")
		}
	}

	result := fmt.Sprintf("预订超时: 创建 %d 条待办", taskCount)
	logger.Log.Debug().Int("task_count", taskCount).Msg("AutoCheckOverdueReservations: 执行完毕")
	return result
}
