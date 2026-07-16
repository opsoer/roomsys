// Package handlers 处理用户认证相关接口，包括登录、用户管理、令牌刷新等
package handlers

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var (
	rateLimitMu   sync.Mutex
	rateLimitData = make(map[string]*rateLimitEntry)
)

const maxLoginAttempts = 10
const rateLimitWindow = 1 * time.Minute
const cleanupInterval = 5 * time.Minute

// init 启动登录频率限制的定时清理任务
func init() {
	go cleanupLoginAttempts()
}

// cleanupLoginAttempts 定期清理过期的登录频率限制记录
func cleanupLoginAttempts() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		rateLimitMu.Lock()
		for ip, entry := range rateLimitData {
			if now.Sub(entry.windowStart) > rateLimitWindow*2 {
				delete(rateLimitData, ip)
			}
		}
		rateLimitMu.Unlock()
	}
}

// checkLoginRateLimit 检查指定IP的登录频率是否超限
func checkLoginRateLimit(ip string) bool {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	now := time.Now()
	if entry, ok := rateLimitData[ip]; ok {
		if now.Sub(entry.windowStart) < rateLimitWindow {
			if entry.count >= maxLoginAttempts {
				return false
			}
			entry.count++
			return true
		}
		entry.windowStart = now
		entry.count = 1
		return true
	}
	rateLimitData[ip] = &rateLimitEntry{windowStart: now, count: 1}
	return true
}

// rateLimitEntry 登录频率限制条目，记录窗口起始时间和请求次数
type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

// AuthHandler 认证处理器，依赖数据库连接、配置和认证服务
type AuthHandler struct {
	DB           *gorm.DB
	Cfg          *config.Config
	AuthService  *services.AuthService
}

// LoginReq 登录请求参数
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 处理用户登录，验证用户名密码并返回令牌
func (h *AuthHandler) Login(c *gin.Context) {
	ip := c.ClientIP()
	if !checkLoginRateLimit(ip) {
		logger.Log.Warn().Str("ip", ip).Msg("登录失败: 频率超限")
		utils.Error(c, http.StatusTooManyRequests, "登录过于频繁，请稍后再试")
		return
	}
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Str("ip", ip).Msg("登录请求参数错误")
		utils.Error(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	user, err := h.AuthService.GetUserByUsername(req.Username)
	if err != nil {
		logger.Log.Warn().Str("username", req.Username).Str("ip", ip).Msg("登录失败: 账号不存在")
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if !h.AuthService.CheckPassword(user.PasswordHash, req.Password) {
		logger.Log.Warn().Str("username", req.Username).Str("ip", ip).Msg("登录失败: 密码错误")
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
		building, err := h.AuthService.GetBuildingByID(buildingID)
		if err == nil && h.AuthService.IsBuildingExpired(building) {
			logger.Log.Warn().Str("username", req.Username).Uint("building_id", buildingID).Msg("登录失败: 公寓已到期")
			utils.Error(c, http.StatusForbidden, "公寓已到期，请联系主理人续费")
			return
		}
	}
	token, err := h.AuthService.GenerateToken(user)
	if err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("生成令牌失败")
		utils.Error(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}
	logger.Log.Info().
		Uint("user_id", user.ID).
		Str("username", user.Username).
		Str("role", user.Role).
		Uint("building_id", buildingID).
		Str("ip", c.ClientIP()).
		Msg("登录成功")
	refreshToken, err := h.AuthService.GenerateRefreshToken(user)
	if err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("生成刷新令牌失败")
		utils.Error(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}
	utils.Success(c, gin.H{
		"token":         token,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"role":        user.Role,
			"building_id": buildingID,
		},
	})
}

// CreateAdminReq 创建公寓管理员请求参数
type CreateAdminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateAdmin 创建公寓管理员（超级管理员专用）
func (h *AuthHandler) CreateAdmin(c *gin.Context) {
	var req CreateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建管理员请求参数错误")
		utils.Error(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	hash, err := h.AuthService.HashPassword(req.Password)
	if err != nil {
		logger.Log.Error().Err(err).Msg("密码加密失败")
		utils.Error(c, http.StatusInternalServerError, "加密失败")
		return
	}
	user := &models.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         "building_admin",
	}
	if err := h.AuthService.CreateUser(user); err != nil {
		logger.Log.Warn().Str("username", req.Username).Msg("创建管理员失败: 用户名已存在")
		utils.Error(c, http.StatusConflict, "用户名已存在")
		return
	}
	logger.Log.Info().Uint("user_id", user.ID).Str("username", user.Username).Str("role", user.Role).Msg("公寓管理员创建成功")
	utils.Created(c, "公寓管理员创建成功", gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

// CreateBuildingAdminReq 创建指定公寓的管理员请求参数
type CreateBuildingAdminReq struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	BuildingID uint   `json:"building_id" binding:"required"`
}

// CreateBuildingAdmin 创建指定公寓的管理员（超级管理员专用）
func (h *AuthHandler) CreateBuildingAdmin(c *gin.Context) {
	var req CreateBuildingAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建公寓管理员请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	hash, err := h.AuthService.HashPassword(req.Password)
	if err != nil {
		logger.Log.Error().Err(err).Msg("密码加密失败")
		utils.Error(c, http.StatusInternalServerError, "加密失败")
		return
	}
	_, err = h.AuthService.GetBuildingByID(req.BuildingID)
	if err != nil {
		logger.Log.Warn().Uint("building_id", req.BuildingID).Msg("创建公寓管理员失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	user := &models.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         "building_admin",
		BuildingID:   &req.BuildingID,
	}
	if err := h.AuthService.CreateUser(user); err != nil {
		logger.Log.Warn().Str("username", req.Username).Msg("创建公寓管理员失败: 用户名已存在")
		utils.Error(c, http.StatusConflict, "用户名已存在")
		return
	}
	logger.Log.Info().Uint("user_id", user.ID).Str("username", user.Username).Uint("building_id", req.BuildingID).Msg("公寓管理员创建成功")
	utils.Created(c, "公寓管理员创建成功", gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role, "building_id": req.BuildingID}})
}

// CreateRegularAdminReq 创建普通管理员请求参数
type CreateRegularAdminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// CreateRegularAdmin 创建当前公寓的普通管理员
func (h *AuthHandler) CreateRegularAdmin(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil || bid == 0 {
		logger.Log.Warn().Msg("创建管理员失败: 未关联公寓")
		utils.Error(c, http.StatusForbidden, "未关联公寓")
		return
	}
	var req CreateRegularAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建管理员请求参数错误")
		utils.Error(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	hash, err := h.AuthService.HashPassword(req.Password)
	if err != nil {
		logger.Log.Error().Err(err).Msg("密码加密失败")
		utils.Error(c, http.StatusInternalServerError, "加密失败")
		return
	}
	user := &models.User{
		Username:     req.Username,
		PasswordHash: hash,
		Role:         "admin",
		BuildingID:   &bid,
	}
	if err := h.AuthService.CreateUser(user); err != nil {
		logger.Log.Warn().Str("username", req.Username).Msg("创建管理员失败: 用户名已存在")
		utils.Error(c, http.StatusConflict, "用户名已存在")
		return
	}
	logger.Log.Info().Uint("user_id", user.ID).Str("username", user.Username).Uint("building_id", bid).Msg("管理员创建成功")
	utils.Created(c, "管理员创建成功", gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

// RefreshTokenReq 刷新令牌请求参数
type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken 使用刷新令牌获取新的访问令牌
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "缺少刷新令牌")
		return
	}
	claims, err := utils.ParseToken(req.RefreshToken, h.Cfg.JWTSecret)
	if err != nil {
		logger.Log.Warn().Err(err).Msg("刷新令牌解析失败")
		utils.Error(c, http.StatusUnauthorized, "刷新令牌无效或已过期")
		return
	}
	if !utils.IsRefreshTokenFromClaims(claims) {
		logger.Log.Warn().Uint("user_id", claims.UserID).Msg("刷新令牌类型不匹配")
		utils.Error(c, http.StatusUnauthorized, "无效的刷新令牌")
		return
	}
	token, err := utils.GenerateToken(claims.UserID, claims.Username, claims.Role, h.Cfg.JWTSecret, claims.BuildingID)
	if err != nil {
		logger.Log.Error().Err(err).Uint("user_id", claims.UserID).Msg("重新生成令牌失败")
		utils.Error(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}
	utils.RevokeToken(req.RefreshToken)
	logger.Log.Info().Uint("user_id", claims.UserID).Msg("令牌刷新成功")
	utils.Success(c, gin.H{"token": token})
}

// ListUsers 获取所有用户列表（超级管理员专用）
func (h *AuthHandler) ListUsers(c *gin.Context) {
	users, err := h.AuthService.ListUsers()
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询用户列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询用户列表失败")
		return
	}
	logger.Log.Debug().Int("count", len(users)).Msg("查询用户列表")
	utils.Success(c, gin.H{"users": users})
}

// ListBuildingUsers 获取当前公寓的用户列表
func (h *AuthHandler) ListBuildingUsers(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	var users []models.User
	if err := h.DB.Select("id, username, role, created_at").Where("building_id = ?", bid).Find(&users).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询楼栋用户列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询用户列表失败")
		return
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(users)).Msg("查询楼栋用户列表")
	utils.Success(c, gin.H{"users": users})
}

// UpdateUserReq 更新用户信息请求参数
type UpdateUserReq struct {
	Role     string `json:"role"`
	Password string `json:"password"`
}

// UpdateUser 更新指定用户的角色或密码
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	user, err := h.AuthService.GetUserByID(uint(userID))
	if err != nil {
		logger.Log.Warn().Str("user_id", id).Msg("更新用户失败: 用户不存在")
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}
	var req UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("更新用户请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.Password != "" {
		hash, err := h.AuthService.HashPassword(req.Password)
		if err != nil {
			logger.Log.Error().Err(err).Msg("密码加密失败")
			utils.Error(c, http.StatusInternalServerError, "加密失败")
			return
		}
		updates["password_hash"] = hash
	}
	if err := h.AuthService.UpdateUser(user.ID, updates); err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("更新用户失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	operatorID := c.GetUint("user_id")
	if operatorID == 0 {
		logger.Log.Warn().Uint("user_id", user.ID).Msg("更新用户: 无法获取操作者ID")
	}
	logger.Log.Info().Str("target_id", id).Uint("operator_id", operatorID).Interface("updates", updates).Msg("用户信息更新成功")
	utils.SuccessWithMsg(c, "更新成功", nil)
}

// DeleteUser 删除指定用户（不允许删除超级管理员）
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	user, err := h.AuthService.GetUserByID(uint(userID))
	if err != nil {
		logger.Log.Warn().Str("user_id", id).Msg("删除用户失败: 用户不存在")
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role == "super_admin" {
		logger.Log.Warn().Str("user_id", id).Msg("删除用户失败: 不能删除超级管理员")
		utils.Error(c, http.StatusForbidden, "不能删除超级管理员")
		return
	}
	if err := h.AuthService.DeleteUser(user.ID); err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("删除用户失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	operatorID := c.GetUint("user_id")
	if operatorID == 0 {
		logger.Log.Warn().Uint("user_id", user.ID).Msg("删除用户: 无法获取操作者ID")
	}
	logger.Log.Info().Uint("deleted_id", user.ID).Str("username", user.Username).Uint("operator_id", operatorID).Msg("用户已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}
