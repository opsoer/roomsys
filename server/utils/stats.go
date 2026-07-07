package utils

import (
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var pvChan = make(chan models.PageView, 10000)

func InitStatsWriter(db *gorm.DB) {
	go pvBatchWriter(db)
}

func pvBatchWriter(db *gorm.DB) {
	batch := make([]models.PageView, 0, 100)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case pv := <-pvChan:
			batch = append(batch, pv)
			if len(batch) >= 100 {
				flushPVBatch(db, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushPVBatch(db, batch)
				batch = batch[:0]
			}
		}
	}
}

func flushPVBatch(db *gorm.DB, batch []models.PageView) {
	if len(batch) == 0 {
		return
	}
	if err := db.Create(&batch).Error; err != nil {
		logger.Log.Warn().Err(err).Int("count", len(batch)).Msg("批量写入page_views失败")
	}
}

func RecordPageView(db *gorm.DB, pageType string, resourceID uint, buildingID uint, ip string) {
	select {
	case pvChan <- models.PageView{
		PageType:   pageType,
		ResourceID: resourceID,
		IP:         ip,
		BuildingID: buildingID,
		CreatedAt:  time.Now(),
	}:
	default:
	}
}

func GetRealIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if ip := strings.TrimSpace(parts[0]); ip != "" {
			return ip
		}
	}
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}
	return c.ClientIP()
}
