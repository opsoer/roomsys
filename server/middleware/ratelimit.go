// 中间件包，提供 API 限流功能
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"rental-server/logger"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
)

// apiRateData 存储按 IP 统计的 API 请求频率数据
var apiRateData = make(map[string]*apiRateEntry)

// apiRateMu 保护 apiRateData 的并发安全
var apiRateMu sync.Mutex

// maxAPIRequests 单个 IP 每分钟允许的最大 API 请求数
const maxAPIRequests = 60

// apiRateWindow 频率统计的时间窗口（1 分钟）
const apiRateWindow = 1 * time.Minute

// apiRateEntry 单 IP 的限流记录，包含窗口起始时间和请求计数
type apiRateEntry struct {
	windowStart time.Time
	count       int
}

// init 启动后台协程，定期清理过期的限流记录
func init() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			apiRateMu.Lock()
			now := time.Now()
			for ip, entry := range apiRateData {
				if now.Sub(entry.windowStart) > apiRateWindow*2 {
					delete(apiRateData, ip)
				}
			}
			apiRateMu.Unlock()
		}
	}()
}

// RateLimitMiddleware API 限流中间件，每 IP 每分钟最多 60 次请求
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
			c.Next()
			return
		}
		ip := c.ClientIP()
		apiRateMu.Lock()
		now := time.Now()
		entry, ok := apiRateData[ip]
		if ok && now.Sub(entry.windowStart) < apiRateWindow {
			if entry.count >= maxAPIRequests {
				apiRateMu.Unlock()
				logger.Log.Warn().Str("ip", ip).Msg("API 请求频率超限")
				utils.Error(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
				c.Abort()
				return
			}
			entry.count++
		} else {
			apiRateData[ip] = &apiRateEntry{windowStart: now, count: 1}
		}
		apiRateMu.Unlock()
		c.Next()
	}
}
