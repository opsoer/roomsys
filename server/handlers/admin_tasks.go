// 平台级待办处理器（超级管理员视角：公寓到期提醒、招商申请等）
package handlers

import (
	"net/http"
	"strconv"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// PlatformTaskHandler 平台待办处理器
type PlatformTaskHandler struct {
	DB             *gorm.DB
	TaskService    *services.TaskService
	RecruitService *services.RecruitService
}

// List 分页获取平台级待办列表（支持按状态筛选、简单分页）
func (h *PlatformTaskHandler) List(c *gin.Context) {
	status := c.Query("status")
	page, size := utils.ParsePage(c)
	query := h.DB.Model(&models.Task{}).Where("scope = ?", "platform")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询平台待办总数失败")
		utils.Error(c, http.StatusInternalServerError, "查询待办失败")
		return
	}
	var tasks []models.Task
	err := query.Preload("Building").Preload("Building.Landlords").Preload("Room").
		Order("created_at DESC, id DESC").
		Offset((page - 1) * size).Limit(size).
		Find(&tasks).Error
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询平台待办列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询待办失败")
		return
	}
	utils.Success(c, gin.H{"tasks": tasks, "total": total, "page": page, "size": size})
}

// Count 获取未处理平台待办数量（菜单徽标）
func (h *PlatformTaskHandler) Count(c *gin.Context) {
	var count int64
	if err := h.DB.Model(&models.Task{}).
		Where("scope = ? AND status = ?", "platform", "pending").
		Count(&count).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询平台待办数量失败")
		utils.Error(c, http.StatusInternalServerError, "查询待办数量失败")
		return
	}
	utils.Success(c, gin.H{"count": count})
}

// Process 处理平台待办：招商申请走招募处理流程，其余直接标记完成
func (h *PlatformTaskHandler) Process(c *gin.Context) {
	id := c.Param("id")
	taskID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的任务ID")
		return
	}
	var task models.Task
	if err := h.DB.First(&task, uint(taskID)).Error; err != nil || task.Scope != "platform" {
		logger.Log.Warn().Str("id", id).Msg("处理平台待办失败: 任务不存在")
		utils.Error(c, http.StatusNotFound, "任务不存在")
		return
	}
	if task.Status != "pending" {
		utils.Error(c, http.StatusBadRequest, "任务已处理")
		return
	}

	if task.Type == "recruit" && task.RefID != nil {
		if err := h.RecruitService.Process(*task.RefID); err != nil {
			logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("处理招商申请失败")
			utils.Error(c, http.StatusInternalServerError, "处理失败")
			return
		}
	}

	if err := h.TaskService.Complete(task.ID); err != nil {
		logger.Log.Error().Err(err).Uint("task_id", task.ID).Msg("更新平台待办状态失败")
		utils.Error(c, http.StatusInternalServerError, "处理失败")
		return
	}
	logger.Log.Info().Uint("task_id", task.ID).Str("type", task.Type).Msg("平台待办处理完成")
	utils.SuccessWithMsg(c, "已处理", nil)
}