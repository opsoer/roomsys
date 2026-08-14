// 招募处理器，处理房东/租客招募相关HTTP请求
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

// RecruitHandler 招募处理器
type RecruitHandler struct {
	DB             *gorm.DB
	RecruitService *services.RecruitService
	TaskService    *services.TaskService
}

// Submit 提交招募申请
func (h *RecruitHandler) Submit(c *gin.Context) {
	var req struct {
		Phone   string `json:"phone"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Phone == "" || req.Address == "" {
		utils.Error(c, http.StatusBadRequest, "请填写电话和地址信息")
		return
	}
	sub := &models.RecruitSubmission{
		Phone:   req.Phone,
		Address: req.Address,
		Status:  "pending",
	}
	if err := h.RecruitService.Submit(sub); err != nil {
		logger.Log.Error().Err(err).Msg("创建招募提交记录失败")
		utils.Error(c, http.StatusInternalServerError, "提交失败")
		return
	}
	if h.TaskService != nil {
		refID := sub.ID
		task := models.Task{
			Title:    "收到新的公寓招商申请",
			Type:     "recruit",
			Priority: "medium",
			Status:   "pending",
			Scope:    "platform",
			RefID:    &refID,
			Description: fmt.Sprintf("有意向的房东信息：地址「%s」，联系电话 %s。请尽快联系确认合作。",
				req.Address, req.Phone),
		}
		if err := h.TaskService.Create(&task); err != nil {
			logger.Log.Error().Err(err).Uint("recruit_id", sub.ID).Msg("创建招商待办失败")
		}
	}
	utils.SuccessWithMsg(c, "提交成功，我们会尽快联系您", nil)
}

// List 获取招募申请列表
func (h *RecruitHandler) List(c *gin.Context) {
	subs, err := h.RecruitService.List()
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询招募列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询招募列表失败")
		return
	}
	utils.Success(c, gin.H{"submissions": subs})
}

// Process 将招募标记为已处理
func (h *RecruitHandler) Process(c *gin.Context) {
	id := c.Param("id")
	recruitID, _ := strconv.ParseUint(id, 10, 32)
	if err := h.RecruitService.Process(uint(recruitID)); err != nil {
		logger.Log.Error().Err(err).Str("id", id).Msg("处理招募记录失败")
		utils.Error(c, http.StatusInternalServerError, "处理失败")
		return
	}
	if h.TaskService != nil {
		h.TaskService.DB.Model(&models.Task{}).
			Where("scope = ? AND type = ? AND ref_id = ? AND status = ?",
				"platform", "recruit", uint(recruitID), "pending").
			Update("status", "completed")
	}
	utils.SuccessWithMsg(c, "已处理", nil)
}

// UnprocessedCount 获取未处理的招募数量
func (h *RecruitHandler) UnprocessedCount(c *gin.Context) {
	count, err := h.RecruitService.UnprocessedCount()
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询未处理招募数量失败")
		utils.Error(c, http.StatusInternalServerError, "查询未处理招募数量失败")
		return
	}
	utils.Success(c, gin.H{"count": count})
}
