// 系统处理器，处理系统时间模拟等管理功能
package handlers

import (
	"net/http"
	"time"

	"rental-server/logger"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemHandler 系统处理器
type SystemHandler struct {
	DB              *gorm.DB
	SettingsService *services.SettingsService
}

// SetTimeReq 设置模拟时间的请求参数
type SetTimeReq struct {
	OffsetSeconds int64 `json:"offset_seconds" binding:"required"`
}

// TimeResp 时间响应
type TimeResp struct {
	SimulatedTime string `json:"simulated_time"`
	OffsetSeconds int64  `json:"offset_seconds"`
}

// GetTime 获取当前模拟时间
func (h *SystemHandler) GetTime(c *gin.Context) {
	now := utils.Now()
	offset := utils.GetTimeOffset()
	utils.Success(c, TimeResp{
		SimulatedTime: now.Format(time.RFC3339),
		OffsetSeconds: int64(offset.Seconds()),
	})
}

// SetTime 设置模拟时间偏移（触发合同到期检查）
func (h *SystemHandler) SetTime(c *gin.Context) {
	var req SetTimeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("设置时间请求参数错误")
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.OffsetSeconds < -43200 || req.OffsetSeconds > 43200 {
		logger.Log.Warn().Int64("offset_seconds", req.OffsetSeconds).Msg("设置时间偏移超出范围")
		utils.Error(c, http.StatusBadRequest, "时间偏移量必须在 -720 到 720 分钟之间")
		return
	}
	userID, _ := utils.GetUserID(c)
	utils.SetTimeOffset(time.Duration(req.OffsetSeconds) * time.Second)
	logger.Log.Info().
		Uint("user_id", userID).
		Int64("offset_seconds", req.OffsetSeconds).
		Msg("模拟时间已更新，触发合同到期检查")
	AutoCheckExpiringContracts(h.DB)
	now := utils.Now()
	offset := utils.GetTimeOffset()
	utils.Success(c, TimeResp{
		SimulatedTime: now.Format(time.RFC3339),
		OffsetSeconds: int64(offset.Seconds()),
	})
}
