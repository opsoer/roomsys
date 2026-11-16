// Package handlers 处理用户认证相关接口，包括登录、用户管理、令牌刷新等
package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"rental-server/audit"
	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// rateLimitMu 保护登录频率限制数据的并发安全
var rateLimitMu sync.Mutex

// rateLimitData 存储按 IP 统计的登录尝试数据
var rateLimitData = make(map[string]*rateLimitEntry)

// defaultLoginAttempts 单个 IP 在时间窗口内允许的最大登录尝试次数默认值
// （可在 config.yaml 的 server.login_rate_limit_per_min 配置，
// 历史环境变量 LOGIN_RATE_LIMIT_PER_MIN 在 config 层仍兼容）。
const defaultLoginAttempts = 10

// rateLimitWindow 登录频率统计的时间窗口（1 分钟）
const rateLimitWindow = 1 * time.Minute

// cleanupInterval 清理过期登录频率记录的时间间隔
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

// isLoginRateLimited 检查指定 IP 的登录失败次数是否已超限；limitPerMin <= 0 时用默认值
func isLoginRateLimited(ip string, limitPerMin int) bool {
	if limitPerMin <= 0 {
		limitPerMin = defaultLoginAttempts
	}
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	now := time.Now()
	if entry, ok := rateLimitData[ip]; ok {
		if now.Sub(entry.windowStart) < rateLimitWindow && entry.count >= limitPerMin {
			return true
		}
	}
	return false
}

// recordLoginFailure 记录一次登录失败（仅失败计入限流，成功登录不影响计数）
func recordLoginFailure(ip string) {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	now := time.Now()
	if entry, ok := rateLimitData[ip]; ok {
		if now.Sub(entry.windowStart) < rateLimitWindow {
			entry.count++
			return
		}
	}
	rateLimitData[ip] = &rateLimitEntry{windowStart: now, count: 1}
}

// rateLimitEntry 登录频率限制条目，记录窗口起始时间和请求次数
type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

// AuthHandler 认证处理器，依赖数据库连接、配置和认证服务
type AuthHandler struct {
	DB          *gorm.DB
	Cfg         *config.Config
	AuthService *services.AuthService
}

// LoginReq 登录请求参数
type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 处理用户登录，验证用户名密码并返回令牌
func (h *AuthHandler) Login(c *gin.Context) {
	ip := c.ClientIP()
	if isLoginRateLimited(ip, h.Cfg.Server.LoginRateLimitPerMin) {
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
		recordLoginFailure(ip)
		logger.Log.Warn().Str("username", req.Username).Str("ip", ip).Msg("登录失败: 账号不存在")
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if !h.AuthService.CheckPassword(user.PasswordHash, req.Password) {
		recordLoginFailure(ip)
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
	audit.LogAs(user.ID, user.Username, user.Role, audit.ActionLogin, audit.ResourceUser, audit.ID(user.ID),
		"登录成功 role="+user.Role)
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
	audit.Log(c, audit.ActionCreate, audit.ResourceUser, audit.ID(user.ID),
		"创建公寓管理员 username="+user.Username+" role="+user.Role)
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
	audit.Log(c, audit.ActionCreate, audit.ResourceUser, audit.ID(user.ID),
		"创建公寓管理员 username="+user.Username+" building_id="+strconv.FormatUint(uint64(req.BuildingID), 10))
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
	audit.Log(c, audit.ActionCreate, audit.ResourceUser, audit.ID(user.ID),
		"创建管理员 username="+user.Username+" building_id="+strconv.FormatUint(uint64(bid), 10))
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
	claims, err := utils.ParseToken(req.RefreshToken, h.Cfg.Server.JWTSecret)
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
	token, err := utils.GenerateToken(claims.UserID, claims.Username, claims.Role, h.Cfg.Server.JWTSecret, claims.BuildingID)
	if err != nil {
		logger.Log.Error().Err(err).Uint("user_id", claims.UserID).Msg("重新生成令牌失败")
		utils.Error(c, http.StatusInternalServerError, "生成令牌失败")
		return
	}
	utils.RevokeToken(req.RefreshToken)
	logger.Log.Debug().Uint("user_id", claims.UserID).Msg("令牌刷新成功")
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
	// 只返回展示所需字段：不含密码哈希与令牌相关列；备注/姓名/电话仅后台可见
	if err := h.DB.Select("id, username, role, created_at, display_name, phone, remark").
		Where("building_id = ?", bid).Find(&users).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询楼栋用户列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询用户列表失败")
		return
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(users)).Msg("查询楼栋用户列表")
	utils.Success(c, gin.H{"users": users})
}

// UpdateUserReq 更新用户信息请求参数
type UpdateUserReq struct {
	Role        string  `json:"role"`
	Password    string  `json:"password"`
	DisplayName *string `json:"display_name"`
	Phone       *string `json:"phone"`
	// Remark 账号备注（仅后台可见）：岗位、负责范围等
	Remark *string `json:"remark"`
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
		now := time.Now()
		updates["token_not_before"] = &now
	}
	// 姓名/电话/备注（仅后台可见）：nil=不变，""=清空
	if req.DisplayName != nil {
		updates["display_name"] = utils.ClipText(*req.DisplayName, 50)
	}
	if req.Phone != nil {
		updates["phone"] = utils.ClipText(*req.Phone, 20)
	}
	if req.Remark != nil {
		updates["remark"] = utils.ClipRemark(*req.Remark)
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
	// 只记录变更了哪些字段，绝不记录密码哈希等敏感值
	changed := make([]string, 0, len(updates))
	for k := range updates {
		if k == "password_hash" {
			changed = append(changed, "password")
			continue
		}
		changed = append(changed, k)
	}
	sort.Strings(changed)
	audit.Log(c, audit.ActionUpdate, audit.ResourceUser, audit.ID(user.ID),
		"更新用户字段: "+strings.Join(changed, ","))
	utils.SuccessWithMsg(c, "更新成功", nil)
}

// UpdateBuildingUserReq 楼栋管理员更新本公寓账号信息请求参数。
// 只开放「后台备注类」字段；角色与密码不在此列，避免提权。
type UpdateBuildingUserReq struct {
	DisplayName *string `json:"display_name"`
	Phone       *string `json:"phone"`
	// Remark 账号备注（仅后台可见）：岗位、负责范围等
	Remark *string `json:"remark"`
}

// UpdateBuildingUser 楼栋管理员更新本公寓账号的姓名/电话/备注。
//
// 越权校验：目标用户必须挂在当前公寓名下（building_id 匹配），
// 超级管理员没有 building_id，因此不在本接口的管辖范围内。
// 与超管的 UpdateUser 分开：超管接口在 SuperAdminMiddleware 后面，
// 楼栋管理员调不动；本接口才是楼栋管理员该走的路（创建管理员本来就有公寓版接口）。
func (h *AuthHandler) UpdateBuildingUser(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	var user models.User
	if err := h.DB.Where("id = ? AND building_id = ?", userID, bid).First(&user).Error; err != nil {
		logger.Log.Warn().Str("user_id", id).Uint("building_id", bid).Msg("更新账号失败: 用户不存在或不属于本公寓")
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}
	var req UpdateBuildingUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("更新账号请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		updates["display_name"] = utils.ClipText(*req.DisplayName, 50)
	}
	if req.Phone != nil {
		updates["phone"] = utils.ClipText(*req.Phone, 20)
	}
	if req.Remark != nil {
		updates["remark"] = utils.ClipRemark(*req.Remark)
	}
	if len(updates) == 0 {
		utils.SuccessWithMsg(c, "更新成功", nil)
		return
	}
	if err := h.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("更新账号失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	changed := make([]string, 0, len(updates))
	for k := range updates {
		changed = append(changed, k)
	}
	sort.Strings(changed)
	audit.Log(c, audit.ActionUpdate, audit.ResourceUser, audit.ID(user.ID),
		"更新用户字段: "+strings.Join(changed, ","))
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
	now := time.Now()
	if err := h.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("token_not_before", &now).Error; err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("删除用户失败: 设置令牌失效时间出错")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
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
	audit.Log(c, audit.ActionDelete, audit.ResourceUser, audit.ID(user.ID),
		"删除用户 username="+user.Username+" role="+user.Role)
	utils.SuccessWithMsg(c, "删除成功", nil)
}

// DeleteBuildingUser 楼栋管理员删除本公寓的普通管理员。
//
// 越权与安全约束：
//   - 目标用户必须挂在当前公寓名下（building_id 匹配）；
//   - 只允许删 role=admin 的普通管理员：楼栋管理员本人与超管都不在本接口管辖内，
//     避免把房东自己的账号删掉导致没人能管这套房；
//   - 不允许删除自己。
func (h *AuthHandler) DeleteBuildingUser(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的用户ID")
		return
	}
	if operatorID := c.GetUint("user_id"); operatorID != 0 && uint(operatorID) == uint(userID) {
		utils.Error(c, http.StatusBadRequest, "不能删除自己")
		return
	}
	var user models.User
	if err := h.DB.Where("id = ? AND building_id = ?", userID, bid).First(&user).Error; err != nil {
		logger.Log.Warn().Str("user_id", id).Uint("building_id", bid).Msg("删除账号失败: 用户不存在或不属于本公寓")
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role != "admin" {
		logger.Log.Warn().Str("user_id", id).Str("role", user.Role).Msg("删除账号失败: 角色不允许删除")
		utils.Error(c, http.StatusForbidden, "只能删除普通管理员")
		return
	}
	now := time.Now()
	if err := h.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("token_not_before", &now).Error; err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("删除账号失败: 设置令牌失效时间出错")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := h.AuthService.DeleteUser(user.ID); err != nil {
		logger.Log.Error().Err(err).Uint("user_id", user.ID).Msg("删除账号失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	audit.Log(c, audit.ActionDelete, audit.ResourceUser, audit.ID(user.ID),
		"删除本公寓账号 username="+user.Username+" role="+user.Role)
	utils.SuccessWithMsg(c, "删除成功", nil)
}

// Logout 处理用户登出，将当前令牌加入吊销列表。
// 同时吊销该用户当前记录的 refresh token，防止登出后仍可刷新换取新会话；
// 客户端也可在 body 中显式上送 refresh_token（多端场景吊销指定的一枚）。
func (h *AuthHandler) Logout(c *gin.Context) {
	auth := c.GetHeader("Authorization")
	if auth == "" || len(auth) < 8 || auth[:7] != "Bearer " {
		utils.Error(c, http.StatusBadRequest, "无效的令牌")
		return
	}
	tokenStr := auth[7:]
	// 先解析 claims（此时令牌尚未吊销），用于定位该用户的 refresh token
	claims, parseErr := utils.ParseToken(tokenStr, h.Cfg.Server.JWTSecret)
	utils.RevokeToken(tokenStr)
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		utils.RevokeToken(req.RefreshToken)
	}
	if parseErr == nil && claims.UserID > 0 {
		utils.RevokeUserRefreshToken(claims.UserID)
	}
	logger.Log.Debug().Uint("user_id", c.GetUint("user_id")).Msg("用户登出")
	audit.Log(c, audit.ActionLogout, audit.ResourceUser, audit.ID(c.GetUint("user_id")), "用户登出")
	utils.SuccessWithMsg(c, "登出成功", nil)
}
