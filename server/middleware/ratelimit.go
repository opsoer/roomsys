// 中间件包，提供 API 限流功能
package middleware

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/utils"
	"rental-server/utils/ipwindow"

	"github.com/gin-gonic/gin"
)

// maxAPIRequests 单个 IP 每分钟允许的最大 API 请求数（可用环境变量 RATE_LIMIT_PER_MIN 覆盖）
var maxAPIRequests = func() int {
	if v := os.Getenv("RATE_LIMIT_PER_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 240
}()

// apiRateWindow 频率统计的时间窗口（1 分钟）
const apiRateWindow = time.Minute

// apiLimiter 全局限流计数器：按 IP 分 64 桶，桶内一把锁 + 一张计数表。
// 相比早期「一把全局 Mutex + 一个 map」，锁竞争降到约 1/64，
// 且后台清理每秒只清一个桶，不会长时间阻塞所有请求。
var apiLimiter = ipwindow.New(ipwindow.Config{
	Limit:           int64(maxAPIRequests),
	Window:          apiRateWindow,
	MaxSizePerShard: 4096,
})

// RateLimitMiddleware API 限流中间件，每 IP 每分钟最多 maxAPIRequests 次请求
// /assets/ 静态资源和 /api/media/ 图片视频文件不计入限流（媒体文件请求量大且为只读资源）
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") || strings.HasPrefix(c.Request.URL.Path, "/api/media/") {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if _, allowed := apiLimiter.Hit(ip); !allowed {
			logger.Log.Warn().Str("ip", ip).Msg("API 请求频率超限")
			utils.Error(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}
