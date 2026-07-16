// 工具包，提供页面浏览统计和批量写入功能
package utils

import (
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// pvChan 页面访问记录缓冲通道，容量 10000，避免阻塞请求
var pvChan = make(chan models.PageView, 10000)

// InitStatsWriter 启动后台协程，批量写入页面访问记录
func InitStatsWriter(db *gorm.DB) {
	go pvBatchWriter(db)
}

// pvBatchWriter 后台批量写入协程，每 10 秒或积累 100 条时写入一次
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

// flushPVBatch 将一批页面访问记录写入数据库
func flushPVBatch(db *gorm.DB, batch []models.PageView) {
	if len(batch) == 0 {
		return
	}
	if err := db.Create(&batch).Error; err != nil {
		logger.Log.Warn().Err(err).Int("count", len(batch)).Msg("批量写入page_views失败")
	}
}

// RecordPageView 向管道发送一条页面访问记录
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

// GetRealIP 从请求头中获取真实客户端 IP（优先 X-Forwarded-For / X-Real-IP）
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
