package handlers

import (
	"net/http"
	"sync"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	loginAttempts = sync.Map{}
	rateLimitMu   sync.Mutex
)

const maxLoginAttempts = 10
const rateLimitWindow = 1 * time.Minute

func checkLoginRateLimit(ip string) bool {
	rateLimitMu.Lock()
	defer rateLimitMu.Unlock()
	now := time.Now()
	if val, ok := loginAttempts.Load(ip); ok {
		entry := val.(*rateLimitEntry)
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
	loginAttempts.Store(ip, &rateLimitEntry{windowStart: now, count: 1})
	return true
}

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

type AuthHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

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
	var user models.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		logger.Log.Warn().Str("username", req.Username).Str("ip", ip).Msg("登录失败: 账号不存在")
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		logger.Log.Warn().Str("username", req.Username).Str("ip", ip).Msg("登录失败: 密码错误")
		utils.Error(c, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
		var building models.Building
		if h.DB.First(&building, buildingID).Error == nil {
			if building.ExpiredAt != "" {
				if expDate, err := time.Parse("2006-01-02", building.ExpiredAt); err == nil && utils.Now().After(expDate) {
					logger.Log.Warn().Str("username", req.Username).Uint("building_id", buildingID).Msg("登录失败: 公寓已到期")
					utils.Error(c, http.StatusForbidden, "公寓已到期，请联系主理人续费")
					return
				}
			}
		}
	}
	token, err := utils.GenerateToken(user.ID, user.Username, user.Role, h.Cfg.JWTSecret, buildingID)
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
	refreshToken, err := utils.GenerateRefreshToken(user.ID, user.Username, user.Role, h.Cfg.JWTSecret, buildingID)
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

type CreateAdminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) CreateAdmin(c *gin.Context) {
	var req CreateAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建管理员请求参数错误")
		utils.Error(c, http.StatusBadRequest, "请输入用户名和密码")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error().Err(err).Msg("密码加密失败")
		utils.Error(c, http.StatusInternalServerError, "加密失败")
		return
	}
	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "building_admin",
	}
	if err := h.DB.Create(&user).Error; err != nil {
		logger.Log.Warn().Str("username", req.Username).Msg("创建管理员失败: 用户名已存在")
		utils.Error(c, http.StatusConflict, "用户名已存在")
		return
	}
	logger.Log.Info().Uint("user_id", user.ID).Str("username", user.Username).Str("role", user.Role).Msg("公寓管理员创建成功")
	utils.Created(c, "公寓管理员创建成功", gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

type CreateBuildingAdminReq struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	BuildingID uint   `json:"building_id" binding:"required"`
}

func (h *AuthHandler) CreateBuildingAdmin(c *gin.Context) {
	var req CreateBuildingAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建公寓管理员请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error().Err(err).Msg("密码加密失败")
		utils.Error(c, http.StatusInternalServerError, "加密失败")
		return
	}
	var building models.Building
	if err := h.DB.First(&building, req.BuildingID).Error; err != nil {
		logger.Log.Warn().Uint("building_id", req.BuildingID).Msg("创建公寓管理员失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "building_admin",
		BuildingID:   &req.BuildingID,
	}
	if err := h.DB.Create(&user).Error; err != nil {
		logger.Log.Warn().Str("username", req.Username).Msg("创建公寓管理员失败: 用户名已存在")
		utils.Error(c, http.StatusConflict, "用户名已存在")
		return
	}
	logger.Log.Info().Uint("user_id", user.ID).Str("username", user.Username).Uint("building_id", req.BuildingID).Msg("公寓管理员创建成功")
	utils.Created(c, "公寓管理员创建成功", gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role, "building_id": req.BuildingID}})
}

type CreateRegularAdminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) CreateRegularAdmin(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	if bid, ok := buildingID.(uint); !ok || bid == 0 {
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
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error().Err(err).Msg("密码加密失败")
		utils.Error(c, http.StatusInternalServerError, "加密失败")
		return
	}
	bid := buildingID.(uint)
	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "admin",
		BuildingID:   &bid,
	}
	if err := h.DB.Create(&user).Error; err != nil {
		logger.Log.Warn().Str("username", req.Username).Msg("创建管理员失败: 用户名已存在")
		utils.Error(c, http.StatusConflict, "用户名已存在")
		return
	}
	logger.Log.Info().Uint("user_id", user.ID).Str("username", user.Username).Uint("building_id", bid).Msg("管理员创建成功")
	utils.Created(c, "管理员创建成功", gin.H{"user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

type RefreshTokenReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

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
	logger.Log.Info().Uint("user_id", claims.UserID).Msg("令牌刷新成功")
	utils.Success(c, gin.H{"token": token})
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.DB.Select("id, username, role, building_id, created_at").Find(&users).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询用户列表失败")
	}
	logger.Log.Debug().Int("count", len(users)).Msg("查询用户列表")
	utils.Success(c, gin.H{"users": users})
}

func (h *AuthHandler) ListBuildingUsers(c *gin.Context) {
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
	var users []models.User
	if err := h.DB.Select("id, username, role, created_at").Where("building_id = ?", bid).Find(&users).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询楼栋用户列表失败")
	}
	logger.Log.Debug().Uint("building_id", bid).Int("count", len(users)).Msg("查询楼栋用户列表")
	utils.Success(c, gin.H{"users": users})
}

type UpdateUserReq struct {
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
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
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Log.Error().Err(err).Msg("密码加密失败")
			utils.Error(c, http.StatusInternalServerError, "加密失败")
			return
		}
		updates["password_hash"] = string(hash)
	}
	if err := h.DB.Model(&user).Updates(updates).Error; err != nil {
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

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		logger.Log.Warn().Str("user_id", id).Msg("删除用户失败: 用户不存在")
		utils.Error(c, http.StatusNotFound, "用户不存在")
		return
	}
	if user.Role == "super_admin" {
		logger.Log.Warn().Str("user_id", id).Msg("删除用户失败: 不能删除超级管理员")
		utils.Error(c, http.StatusForbidden, "不能删除超级管理员")
		return
	}
	if err := h.DB.Delete(&user).Error; err != nil {
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
