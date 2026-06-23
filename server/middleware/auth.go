package middleware

import (
	"net/http"
	"strings"

	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AuthMiddleware(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		claims, err := utils.ParseToken(tokenStr, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已过期"})
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("building_id", claims.BuildingID)
		c.Next()
	}
}

func AdminOrBuildingAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" && role != "building_admin" && role != "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权限，仅管理员可操作"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func SuperAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权限，仅超级管理员可操作"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func BuildingAdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "building_admin" && role != "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "无权限，仅公寓管理员可操作"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func BuildingScopeMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		buildingID, _ := c.Get("building_id")
		if role == "super_admin" {
			c.Next()
			return
		}
		if bid, ok := buildingID.(uint); !ok || bid == 0 {
			c.JSON(http.StatusForbidden, gin.H{"error": "未关联公寓"})
			c.Abort()
			return
		}
		c.Next()
	}
}
