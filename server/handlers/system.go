// 系统处理器，处理系统时间模拟等管理功能
package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"rental-server/audit"
	"rental-server/logger"
	"rental-server/models"
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
	audit.Log(c, audit.ActionConfig, audit.ResourceTime, "simulated",
		"修改模拟时间 偏移秒="+strconv.FormatInt(offsetSeconds, 10))
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
	audit.Log(c, audit.ActionTrigger, audit.ResourceJob, "all", "手动触发全部定时任务")

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
	audit.Log(c, audit.ActionTrigger, audit.ResourceJob, name, "手动触发定时任务")
	result, err := h.Jobs.Run(name)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	utils.Success(c, gin.H{"result": result})
}

// applyAuditFilters 把查询串里的审计筛选条件应用到查询上。
// List 与 Delete 共用，保证「看到什么就能删什么」。
// 时间接受 "2006-01-02 15:04:05" 或 "2006-01-02"（纯日期按当天 00:00 计）。
func applyAuditFilters(c *gin.Context, q *gorm.DB) *gorm.DB {
	if res := c.Query("resource"); res != "" {
		q = q.Where("resource = ?", res)
	}
	if act := c.Query("action"); act != "" {
		q = q.Where("action = ?", act)
	}
	if role := c.Query("role"); role != "" {
		q = q.Where("role = ?", role)
	}
	if username := c.Query("username"); username != "" {
		q = q.Where("username LIKE ?", "%"+strings.TrimSpace(username)+"%")
	}
	if start := parseFlexibleTime(c.Query("start_time")); start != nil {
		q = q.Where("created_at >= ?", *start)
	}
	if end := parseFlexibleTime(c.Query("end_time")); end != nil {
		q = q.Where("created_at < ?", *end)
	}
	if before := parseFlexibleTime(c.Query("before_time")); before != nil {
		q = q.Where("created_at < ?", *before)
	}
	return q
}

// auditFilterDesc 汇总当前请求的筛选条件，用于审计明细留痕。
func auditFilterDesc(c *gin.Context) string {
	parts := make([]string, 0, 6)
	if v := c.Query("resource"); v != "" {
		parts = append(parts, "资源="+v)
	}
	if v := c.Query("action"); v != "" {
		parts = append(parts, "动作="+v)
	}
	if v := c.Query("role"); v != "" {
		parts = append(parts, "角色="+v)
	}
	if v := strings.TrimSpace(c.Query("username")); v != "" {
		parts = append(parts, "操作人="+v)
	}
	if v := c.Query("before_time"); v != "" {
		parts = append(parts, "时间早于="+v)
	}
	if len(parts) == 0 {
		return "无条件"
	}
	return strings.Join(parts, "，")
}

// ListAuditLogs 查询操作审计记录（超级管理员专用）。
// 支持按资源/动作/角色/操作人/时间范围/before_time 过滤与分页。
func (h *SystemHandler) ListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page <= 0 {
		page = 1
	}
	q := applyAuditFilters(c, h.DB.Model(&models.AuditLog{}))
	var total int64
	if err := q.Count(&total).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询审计记录总数失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var logs []models.AuditLog
	if err := q.Order("id DESC").Limit(limit).Offset((page - 1) * limit).Find(&logs).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询审计记录失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"logs": logs, "total": total, "page": page, "limit": limit})
}

// DeleteAuditLogs 批量清理审计记录（超级管理员专用），防止 audit_logs 表无限膨胀。
// 必须带 before_time（删除该时间点之前的记录），可再叠加 resource/action/role/username 收窄范围；
// 本次清理操作本身会写入一条审计留痕。
func (h *SystemHandler) DeleteAuditLogs(c *gin.Context) {
	if parseFlexibleTime(c.Query("before_time")) == nil {
		utils.Error(c, http.StatusBadRequest, "必须提供有效的 before_time（删除该时间之前的记录）")
		return
	}
	desc := auditFilterDesc(c)
	q := applyAuditFilters(c, h.DB.Model(&models.AuditLog{}))
	res := q.Delete(&models.AuditLog{})
	if res.Error != nil {
		logger.Log.Error().Err(res.Error).Msg("清理审计记录失败")
		utils.Error(c, http.StatusInternalServerError, "清理失败")
		return
	}
	logger.Log.Info().Int64("rows", res.RowsAffected).Str("filter", desc).Msg("审计记录已清理")
	audit.Log(c, audit.ActionDelete, audit.ResourceAudit, "cleanup",
		"清理审计记录（"+desc+"）删除 "+strconv.FormatInt(res.RowsAffected, 10)+" 条")
	utils.Success(c, gin.H{"deleted": res.RowsAffected})
}

// parseFlexibleTime 解析筛选时间：支持 "2006-01-02 15:04:05" 与 "2006-01-02"，
// 纯日期按当天 00:00:00 处理；为空或格式错误返回 nil（不参与过滤）。
func parseFlexibleTime(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return &t
		}
	}
	return nil
}
