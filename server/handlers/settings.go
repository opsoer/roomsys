// 设置处理器，处理系统配置的读写请求
package handlers

import (
	"net/http"

	"rental-server/logger"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SettingsHandler 设置处理器
type SettingsHandler struct {
	DB              *gorm.DB
	SettingsService *services.SettingsService
}

// Get 获取指定键的设置值
func (h *SettingsHandler) Get(c *gin.Context) {
	key := c.Param("key")
	setting, err := h.SettingsService.Get(key)
	if err != nil {
		utils.Success(c, gin.H{"key": key, "value": ""})
		return
	}
	utils.Success(c, gin.H{"key": setting.Key, "value": setting.Value})
}

// Update 更新指定键的设置值
func (h *SettingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的请求")
		return
	}
	if err := h.SettingsService.Update(key, body.Value); err != nil {
		logger.Log.Error().Err(err).Str("key", key).Msg("更新设置失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.Success(c, gin.H{"key": key, "value": body.Value})
}


