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

var (
	apiRateData = make(map[string]*apiRateEntry)
	apiRateMu   sync.Mutex
)

const maxAPIRequests = 60
const apiRateWindow = 1 * time.Minute

type apiRateEntry struct {
	windowStart time.Time
	count       int
}

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
