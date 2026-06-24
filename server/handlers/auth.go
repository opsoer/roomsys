package handlers

import (
	"net/http"
	"time"

	"rental-server/config"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	var user models.User
	if err := h.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号不存在"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
		var building models.Building
		if h.DB.First(&building, buildingID).Error == nil {
			if building.ExpiredAt != "" {
				if expDate, err := time.Parse("2006-01-02", building.ExpiredAt); err == nil && utils.Now().After(expDate) {
					c.JSON(http.StatusForbidden, gin.H{"error": "公寓已到期，请联系主理人续费"})
					return
				}
			}
		}
	}
	token, err := utils.GenerateToken(user.ID, user.Username, user.Role, h.Cfg.JWTSecret, buildingID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成令牌失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":          user.ID,
			"username":    user.Username,
			"role":        user.Role,
			"building_id": buildingID,
		},
	})
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}
	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "注册成功"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	buildingID := uint(0)
	if user.BuildingID != nil {
		buildingID = *user.BuildingID
	}
	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}
	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "building_admin",
	}
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "公寓管理员创建成功", "user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

type CreateBuildingAdminReq struct {
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password" binding:"required"`
	BuildingID uint   `json:"building_id" binding:"required"`
}

func (h *AuthHandler) CreateBuildingAdmin(c *gin.Context) {
	var req CreateBuildingAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
		return
	}
	var building models.Building
	if err := h.DB.First(&building, req.BuildingID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
	user := models.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		Role:         "building_admin",
		BuildingID:   &req.BuildingID,
	}
	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "公寓管理员创建成功", "user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role, "building_id": req.BuildingID}})
}

type CreateRegularAdminReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) CreateRegularAdmin(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	if bid, ok := buildingID.(uint); !ok || bid == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "未关联公寓"})
		return
	}
	var req CreateRegularAdminReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入用户名和密码"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加密失败"})
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
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "管理员创建成功", "user": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []models.User
	h.DB.Select("id, username, role, building_id, created_at").Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *AuthHandler) ListBuildingUsers(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var users []models.User
	h.DB.Select("id, username, role, created_at").Where("building_id = ?", bid).Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users})
}

type UpdateUserReq struct {
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	var req UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]interface{}{}
	if req.Role != "" {
		updates["role"] = req.Role
	}
	if req.Password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		updates["password_hash"] = string(hash)
	}
	h.DB.Model(&user).Updates(updates)
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")
	var user models.User
	if err := h.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	if user.Role == "super_admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "不能删除超级管理员"})
		return
	}
	h.DB.Delete(&user)
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}
