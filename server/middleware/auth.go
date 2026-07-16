// 中间件包，提供认证、授权和权限控制相关中间件
package middleware

import (
	"net/http"
	"strings"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuthMiddleware JWT 认证中间件，验证请求头中的 Bearer Token
func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			logger.Log.Warn().Str("ip", c.ClientIP()).Str("path", c.Request.URL.Path).Msg("未登录请求")
			utils.Error(c, http.StatusUnauthorized, "未登录")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := utils.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			logger.Log.Warn().Err(err).Str("ip", c.ClientIP()).Msg("JWT 解析失败")
			utils.Error(c, http.StatusUnauthorized, "登录已过期")
			c.Abort()
			return
		}
		logger.Log.Debug().
			Uint("user_id", claims.UserID).
			Str("username", claims.Username).
			Str("role", claims.Role).
			Uint("building_id", claims.BuildingID).
			Str("path", c.Request.URL.Path).
			Msg("请求认证通过")
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("building_id", claims.BuildingID)
		c.Next()
	}
}

// AdminOrBuildingAdminMiddleware 管理员或楼管权限中间件
func AdminOrBuildingAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		userIDVal, _ := c.Get("user_id")
		role, roleOk := roleVal.(string)
		userID, userOk := userIDVal.(uint)
		if !exists || !roleOk || !userOk {
			utils.Error(c, http.StatusUnauthorized, "未授权")
			c.Abort()
			return
		}
		if role != "admin" && role != "building_admin" && role != "super_admin" {
			logger.Log.Warn().
				Uint("user_id", userID).
				Str("role", role).
				Str("path", c.Request.URL.Path).
				Msg("权限不足: 仅管理员可操作")
			utils.Error(c, http.StatusForbidden, "无权限，仅管理员可操作")
			c.Abort()
			return
		}
		c.Next()
	}
}

// SuperAdminMiddleware 超级管理员权限中间件
func SuperAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role")
		userIDVal, _ := c.Get("user_id")
		role, roleOk := roleVal.(string)
		userID, userOk := userIDVal.(uint)
		if !exists || !roleOk || !userOk {
			utils.Error(c, http.StatusUnauthorized, "未授权")
			c.Abort()
			return
		}
		if role != "super_admin" {
			logger.Log.Warn().
				Uint("user_id", userID).
				Str("role", role).
				Str("path", c.Request.URL.Path).
				Msg("权限不足: 仅超级管理员可操作")
			utils.Error(c, http.StatusForbidden, "无权限，仅超级管理员可操作")
			c.Abort()
			return
		}
		c.Next()
	}
}

// FullPackageMiddleware 全套餐权限中间件，检查楼宇是否购买了全功能套餐
func FullPackageMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		buildingID, _ := c.Get("building_id")
		bid, ok := buildingID.(uint)
		if !ok || bid == 0 {
			utils.Error(c, http.StatusForbidden, "未关联公寓"); c.Abort()
			return
		}
		var building models.Building
		if err := db.First(&building, bid).Error; err != nil {
			utils.Error(c, http.StatusNotFound, "公寓不存在"); c.Abort()
			return
		}
		if building.Package != "full" {
			logger.Log.Warn().
				Uint("building_id", bid).
				Str("package", building.Package).
				Str("path", c.Request.URL.Path).
				Msg("套餐不支持此功能")
			utils.Error(c, http.StatusForbidden, "当前套餐不支持此功能，请升级为全套餐")
			c.Abort()
			return
		}
		c.Next()
	}
}

// BuildingScopeMiddleware 楼宇数据范围中间件，确保用户只能访问所属楼宇的数据
func BuildingScopeMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, _ := c.Get("role")
		buildingID, _ := c.Get("building_id")
		userIDVal, _ := c.Get("user_id")
		role, roleOk := roleVal.(string)
		userID, userOk := userIDVal.(uint)
		if !roleOk || !userOk {
			utils.Error(c, http.StatusUnauthorized, "未授权")
			c.Abort()
			return
		}
		if role == "super_admin" {
			c.Next()
			return
		}
		bid, ok := buildingID.(uint)
		if !ok || bid == 0 {
			logger.Log.Warn().
				Uint("user_id", userID).
				Str("role", role).
				Str("path", c.Request.URL.Path).
				Msg("用户未关联公寓")
			utils.Error(c, http.StatusForbidden, "未关联公寓")
			c.Abort()
			return
		}
		var user models.User
		if err := db.First(&user, userID).Error; err != nil {
			logger.Log.Error().Err(err).Uint("user_id", userID).Msg("验证用户building归属失败")
			utils.Error(c, http.StatusInternalServerError, "验证失败")
			c.Abort()
			return
		}
		if user.BuildingID == nil || *user.BuildingID != bid {
			logger.Log.Warn().
				Uint("user_id", userID).
				Uint("jwt_building_id", bid).
				Msg("用户building归属验证失败")
			utils.Error(c, http.StatusForbidden, "无权访问该公寓")
			c.Abort()
			return
		}
		c.Next()
	}
}
