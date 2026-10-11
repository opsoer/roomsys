// 工具包，提供页面浏览统计和批量写入功能
package utils

import (
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

// GetRealIP 获取客户端真实 IP。
// 直接返回 c.ClientIP()，由 Gin 的 TrustedProxies 配置决定是否采信 X-Forwarded-For。
// 部署时需在 main.go 中调用 r.SetTrustedProxies() 指定可信反代 IP，防止伪造 XFF。
func GetRealIP(c *gin.Context) string {
	return c.ClientIP()
}
