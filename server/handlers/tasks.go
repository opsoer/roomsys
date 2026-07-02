package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct {
	DB          *gorm.DB
	TaskService *services.TaskService
}

type ProcessTaskReq struct {
	RefundedDeposit float64 `json:"refunded_deposit"`
}

func (h *TaskHandler) List(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	status := c.Query("status")
	tasks, err := h.TaskService.List(bid, status)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询任务列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询任务列表失败")
		return
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(tasks)).Msg("查询任务列表")
	utils.Success(c, gin.H{"tasks": tasks})
}

func (h *TaskHandler) Process(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	taskID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的任务ID")
		return
	}
	task, err := h.TaskService.GetByID(uint(taskID))
	if err != nil || task.BuildingID != bid {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("处理任务失败: 任务不存在")
		utils.Error(c, http.StatusNotFound, "任务不存在")
		return
	}
	if task.Type != "expired_room" {
		logger.Log.Warn().Uint("task_id", task.ID).Str("type", task.Type).Msg("处理任务失败: 不支持的操作")
		utils.Error(c, http.StatusBadRequest, "该任务不支持此操作")
		return
	}
	if task.Status != "pending" {
		logger.Log.Warn().Uint("task_id", task.ID).Msg("处理任务失败: 任务已处理")
		utils.Error(c, http.StatusBadRequest, "任务已处理")
		return
	}

	var req ProcessTaskReq
	if err := c.ShouldBindJSON(&req); err != nil || req.RefundedDeposit < 0 {
		logger.Log.Warn().Uint("task_id", task.ID).Msg("处理任务失败: 缺少退还押金金额")
		utils.Error(c, http.StatusBadRequest, "请填写退还押金金额")
		return
	}

	if task.RoomID == nil {
		logger.Log.Warn().Uint("task_id", task.ID).Msg("处理任务失败: 关联房间不存在")
		utils.Error(c, http.StatusNotFound, "关联房间不存在")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		} else if tx.Error != nil {
			tx.Rollback()
		}
	}()

	if err := handleDepositRefund(tx, models.Room{ID: *task.RoomID}, req.RefundedDeposit, uid, bid); err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("创建押金退还账单失败")
		utils.Error(c, http.StatusInternalServerError, "创建退还账单失败")
		return
	}

	if err := tx.Model(&models.Room{}).Where("id = ? AND status != ?", *task.RoomID, "vacant").Update("status", "vacant").Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("room_id", *task.RoomID).Msg("更新房间状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新房间状态失败")
		return
	}
	if err := tx.Model(&models.RentalContract{}).Where("room_id = ? AND status = ?", *task.RoomID, "active").Update("status", "ended").Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("room_id", *task.RoomID).Msg("结束合同失败")
		utils.Error(c, http.StatusInternalServerError, "结束合同失败")
		return
	}
	if err := tx.Model(&models.Task{}).Where("id = ?", task.ID).Update("status", "completed").Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("更新任务状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新任务状态失败")
		return
	}
	tx.Commit()
	logger.Log.Info().
		Uint("task_id", task.ID).
		Uint("room_id", *task.RoomID).
		Uint("building_id", bid).
		Float64("refunded_deposit", req.RefundedDeposit).
		Msg("退租任务处理完成")
	utils.SuccessWithMsg(c, "退租处理完成", nil)
}

func (h *TaskHandler) Complete(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	taskID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的任务ID")
		return
	}
	task, err := h.TaskService.GetByID(uint(taskID))
	if err != nil || task.BuildingID != bid {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("完成任务失败: 任务不存在")
		utils.Error(c, http.StatusNotFound, "任务不存在")
		return
	}
	if err := h.TaskService.Complete(task.ID); err != nil {
		logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("更新任务状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新任务状态失败")
		return
	}
	logger.Log.Info().Uint("task_id", task.ID).Str("title", task.Title).Msg("任务标记完成")
	utils.SuccessWithMsg(c, "任务已完成", nil)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	taskID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的任务ID")
		return
	}
	if err := h.TaskService.Delete(uint(taskID)); err != nil {
		logger.Log.Error().Err(err).Str("id", id).Uint("building_id", bid).Msg("删除任务失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Str("id", id).Uint("building_id", bid).Msg("任务已删除")
	utils.SuccessWithMsg(c, "已删除", nil)
}

func handleDepositRefund(tx *gorm.DB, room models.Room, refundedDeposit float64, userID, buildingID uint) error {
	if refundedDeposit <= 0 {
		return nil
	}

	now := utils.Now()
	datePart := now.Format("20060102")
	var count int64
	tx.Model(&models.Bill{}).
		Where("building_id = ? AND bill_no LIKE ?", buildingID, "B"+datePart+"%").
		Count(&count)
	billNo := fmt.Sprintf("B%s%05d", datePart, count+1)

	billDate := now.Format("2006-01-02")
	bill := models.Bill{
		BuildingID:  buildingID,
		BillNo:      billNo,
		Type:        "expense",
		Subtype:     "押金退还",
		Amount:      refundedDeposit,
		RoomID:      &room.ID,
		Description: "退租押金退还",
		BillDate:    billDate,
		CreatedBy:   userID,
	}
	return tx.Create(&bill).Error
}
