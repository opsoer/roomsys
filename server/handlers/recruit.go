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

type RecruitHandler struct {
	DB             *gorm.DB
	RecruitService *services.RecruitService
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
	utils.SuccessWithMsg(c, "提交成功，我们会尽快联系您", nil)
}

func (h *RecruitHandler) List(c *gin.Context) {
	subs, err := h.RecruitService.List()
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询招募列表失败")
	}
	utils.Success(c, gin.H{"submissions": subs})
}

func (h *RecruitHandler) Process(c *gin.Context) {
	id := c.Param("id")
	recruitID, _ := strconv.ParseUint(id, 10, 32)
	if err := h.RecruitService.Process(uint(recruitID)); err != nil {
		logger.Log.Error().Err(err).Str("id", id).Msg("处理招募记录失败")
		utils.Error(c, http.StatusInternalServerError, "处理失败")
		return
	}
	utils.SuccessWithMsg(c, "已处理", nil)
}

func (h *RecruitHandler) UnprocessedCount(c *gin.Context) {
	count, err := h.RecruitService.UnprocessedCount()
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询未处理招募数量失败")
	}
	utils.Success(c, gin.H{"count": count})
}
