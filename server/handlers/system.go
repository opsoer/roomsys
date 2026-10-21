// 系统处理器，处理系统时间模拟等管理功能
package handlers

import (
	"net/http"
	"time"

	"rental-server/logger"
	"rental-server/scheduler"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SystemHandler 系统处理器
type SystemHandler struct {
	DB              *gorm.DB
	SettingsService *services.SettingsService
	// Jobs 定时任务注册表：列表查看与单个/全部手动执行共用
	Jobs *scheduler.Registry
}

// SetTimeReq 设置模拟时间的请求参数
type SetTimeReq struct {
	OffsetSeconds *int64 `json:"offset_seconds"`
	TargetTime    string `json:"target_time"`
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

// SetTime 设置模拟时间（偏移模式或指定时间模式）
func (h *SystemHandler) SetTime(c *gin.Context) {
	var req SetTimeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("设置时间请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数格式错误")
		return
	}

	var offsetSeconds int64
	if req.TargetTime != "" {
		layout := "2006-01-02 15:04:05"
		target, err := time.ParseInLocation(layout, req.TargetTime, time.Local)
		if err != nil {
			logger.Log.Warn().Str("target_time", req.TargetTime).Msg("指定时间格式错误")
			utils.Error(c, http.StatusBadRequest, "时间格式错误，请使用 YYYY-MM-DD HH:mm:ss")
			return
		}
		offsetSeconds = int64(target.Sub(time.Now()).Seconds())
	} else if req.OffsetSeconds != nil {
		offsetSeconds = *req.OffsetSeconds
	} else {
		utils.Error(c, http.StatusBadRequest, "请提供 offset_seconds 或 target_time")
		return
	}

	userID, _ := utils.GetUserID(c)
	utils.SetTimeOffset(time.Duration(offsetSeconds) * time.Second)
	logger.Log.Info().
		Uint("user_id", userID).
		Int64("offset_seconds", offsetSeconds).
		Msg("模拟时间已更新，触发合同到期检查")
	if _, err := h.Jobs.Run("contract_expiry"); err != nil {
		logger.Log.Warn().Err(err).Msg("模拟时间变更后触发合同检查失败")
	}
	now := utils.Now()
	offset := utils.GetTimeOffset()
	utils.Success(c, TimeResp{
		SimulatedTime: now.Format(time.RFC3339),
		OffsetSeconds: int64(offset.Seconds()),
	})
}

// RunTasks 手动触发所有定时任务。
// 便于在调整系统时间后无需等待定时器即可立即看到结果。
func (h *SystemHandler) RunTasks(c *gin.Context) {
	userID, _ := utils.GetUserID(c)
	logger.Log.Info().Uint("user_id", userID).Msg("手动触发全部定时任务")

	utils.Success(c, gin.H{"results": h.Jobs.RunAll()})
}

// ListJobs 列出全部定时任务（含调度配置与最近一次执行记录）。
func (h *SystemHandler) ListJobs(c *gin.Context) {
	utils.Success(c, gin.H{"jobs": h.Jobs.List()})
}

// RunJob 手动执行单个定时任务（不受配置 enabled 限制，便于验证）。
func (h *SystemHandler) RunJob(c *gin.Context) {
	name := c.Param("name")
	userID, _ := utils.GetUserID(c)
	logger.Log.Info().Uint("user_id", userID).Str("job", name).Msg("手动触发定时任务")
	result, err := h.Jobs.Run(name)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"result": result})
}
