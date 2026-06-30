package handlers

import (
	"net/http"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SettingsHandler struct {
	DB *gorm.DB
}

func (h *SettingsHandler) Get(c *gin.Context) {
	key := c.Param("key")
	var s models.Setting
	if err := h.DB.Where(&models.Setting{Key: key}).First(&s).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "设置不存在")
		return
	}
	utils.Success(c, gin.H{"key": s.Key, "value": s.Value})
}

func (h *SettingsHandler) Update(c *gin.Context) {
	key := c.Param("key")
	var body struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的请求")
		return
	}
	var s models.Setting
	if err := h.DB.Where(&models.Setting{Key: key}).Assign(models.Setting{Value: body.Value}).FirstOrCreate(&s).Error; err != nil {
		logger.Log.Error().Err(err).Str("key", key).Msg("更新设置失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.Success(c, gin.H{"key": key, "value": body.Value})
}

func (h *SettingsHandler) GetPublicRecruit(c *gin.Context) {
	phone := ""
	var s models.Setting
	if err := h.DB.Where(&models.Setting{Key: "recruit_phone"}).First(&s).Error; err == nil {
		phone = s.Value
	}
	utils.Success(c, gin.H{"phone": phone})
}
