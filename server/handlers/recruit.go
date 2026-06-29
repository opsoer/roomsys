package handlers

import (
	"net/http"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type RecruitHandler struct {
	DB *gorm.DB
}

func (h *RecruitHandler) Submit(c *gin.Context) {
	var req struct {
		Phone   string `json:"phone"`
		Address string `json:"address"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Phone == "" || req.Address == "" {
		utils.Error(c, http.StatusBadRequest, "请填写电话和地址信息")
		return
	}
	sub := models.RecruitSubmission{
		Phone:   req.Phone,
		Address: req.Address,
		Status:  "pending",
	}
	if err := h.DB.Create(&sub).Error; err != nil {
		logger.Log.Error().Err(err).Msg("创建招募提交记录失败")
		utils.Error(c, http.StatusInternalServerError, "提交失败")
		return
	}
	utils.SuccessWithMsg(c, "提交成功，我们会尽快联系您", nil)
}

func (h *RecruitHandler) List(c *gin.Context) {
	var subs []models.RecruitSubmission
	if err := h.DB.Order("created_at desc").Find(&subs).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询招募列表失败")
	}
	utils.Success(c, gin.H{"submissions": subs})
}

func (h *RecruitHandler) Process(c *gin.Context) {
	id := c.Param("id")
	var sub models.RecruitSubmission
	if err := h.DB.First(&sub, id).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "记录不存在")
		return
	}
	if err := h.DB.Model(&sub).Update("status", "processed").Error; err != nil {
		logger.Log.Error().Err(err).Uint("id", sub.ID).Msg("处理招募记录失败")
		utils.Error(c, http.StatusInternalServerError, "处理失败")
		return
	}
	utils.SuccessWithMsg(c, "已处理", nil)
}

func (h *RecruitHandler) UnprocessedCount(c *gin.Context) {
	var count int64
	if err := h.DB.Model(&models.RecruitSubmission{}).Where("status = ?", "pending").Count(&count).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询未处理招募数量失败")
	}
	utils.Success(c, gin.H{"count": count})
}
