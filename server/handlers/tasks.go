package handlers

import (
	"net/http"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct {
	DB *gorm.DB
}

type ProcessTaskReq struct {
	RefundedDeposit float64 `json:"refunded_deposit"`
}

func (h *TaskHandler) List(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	var tasks []models.Task
	query := h.DB.Preload("Room").Where("building_id = ?", bid).Order("created_at desc")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Find(&tasks).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询任务列表失败")
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(tasks)).Msg("查询任务列表")
	utils.Success(c, gin.H{"tasks": tasks})
}

func (h *TaskHandler) Process(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	id := c.Param("id")
	var task models.Task
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&task).Error; err != nil {
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

	var room models.Room
	if task.RoomID == nil || h.DB.Where("id = ? AND building_id = ?", *task.RoomID, bid).First(&room).Error != nil {
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
	handleDepositRefund(h.DB, room, req.RefundedDeposit, uid, bid)

	if err := h.DB.Model(&room).Where("status != ?", "vacant").Update("status", "vacant").Error; err != nil {
		logger.Log.Error().Err(err).Uint("room_id", room.ID).Msg("更新房间状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新房间状态失败")
		return
	}
	if err := h.DB.Model(&task).Update("status", "completed").Error; err != nil {
		logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("更新任务状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新任务状态失败")
		return
	}
	logger.Log.Info().
		Uint("task_id", task.ID).
		Uint("room_id", room.ID).
		Uint("building_id", bid).
		Float64("refunded_deposit", req.RefundedDeposit).
		Msg("退租任务处理完成")
	utils.SuccessWithMsg(c, "退租处理完成", nil)
}

func (h *TaskHandler) Complete(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	id := c.Param("id")
	var task models.Task
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&task).Error; err != nil {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("完成任务失败: 任务不存在")
		utils.Error(c, http.StatusNotFound, "任务不存在")
		return
	}
	if err := h.DB.Model(&task).Update("status", "completed").Error; err != nil {
		logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("更新任务状态失败")
		utils.Error(c, http.StatusInternalServerError, "更新任务状态失败")
		return
	}
	logger.Log.Info().Uint("task_id", task.ID).Str("title", task.Title).Msg("任务标记完成")
	utils.SuccessWithMsg(c, "任务已完成", nil)
}

func (h *TaskHandler) Delete(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	id := c.Param("id")
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).Delete(&models.Task{}).Error; err != nil {
		logger.Log.Error().Err(err).Str("id", id).Uint("building_id", bid).Msg("删除任务失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Str("id", id).Uint("building_id", bid).Msg("任务已删除")
	utils.SuccessWithMsg(c, "已删除", nil)
}
