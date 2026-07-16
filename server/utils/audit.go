// 工具包，提供审计日志记录功能
package utils

import (
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LogAudit 记录操作审计日志到数据库
func LogAudit(c *gin.Context, db *gorm.DB, action, resource, resourceID, detail string) {
	userID, _ := c.Get("user_id")
	uid, _ := userID.(uint)
	username, _ := c.Get("username")
	name, _ := username.(string)
	buildingID, _ := c.Get("building_id")
	bid, _ := buildingID.(uint)

	entry := models.AuditLog{
		UserID:     uid,
		Username:   name,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
		IP:         c.ClientIP(),
	}
	if bid > 0 {
		entry.BuildingID = &bid
	}
	if err := db.Create(&entry).Error; err != nil {
		_ = err
	}
}
