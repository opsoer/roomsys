// 中间件包，提供 API 限流功能
package middleware

import (
	"net/http"
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/utils"
	"rental-server/utils/ipwindow"

	"github.com/gin-gonic/gin"
)

// defaultAPIRequests 单个 IP 每分钟允许的最大 API 请求数默认值
// （可在 config.yaml 的 server.rate_limit_per_min 配置，历史环境变量 RATE_LIMIT_PER_MIN 仍兼容）。
const defaultAPIRequests = 240

// apiRateWindow 频率统计的时间窗口（1 分钟）
const apiRateWindow = time.Minute

// RateLimitMiddleware API 限流中间件，每 IP 每分钟最多 limitPerMin 次请求；
// limitPerMin <= 0 时使用默认值（配置层已兜底，这里再防一次，供未走 normalize 的调用方）。
// /assets/ 静态资源和 /api/media/ 图片视频文件不计入限流（媒体文件请求量大且为只读资源）。
// 限流计数器随中间件创建（每个引擎一个），关闭逻辑由调用方持有返回的关闭函数或随进程退出。
func RateLimitMiddleware(limitPerMin int) gin.HandlerFunc {
	if limitPerMin <= 0 {
		limitPerMin = defaultAPIRequests
	}
	// 限流计数器：按 IP 分 64 桶，桶内一把锁 + 一张计数表。
	// 相比早期「一把全局 Mutex + 一个 map」，锁竞争降到约 1/64，
	// 且后台清理每秒只清一个桶，不会长时间阻塞所有请求。
	limiter := ipwindow.New(ipwindow.Config{
		Limit:           int64(limitPerMin),
		Window:          apiRateWindow,
		MaxSizePerShard: 4096,
	})
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/assets/") || strings.HasPrefix(c.Request.URL.Path, "/api/media/") {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if _, allowed := limiter.Hit(ip); !allowed {
			logger.Log.Warn().Str("ip", ip).Msg("API 请求频率超限")
			utils.Error(c, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
			c.Abort()
			return
		}
		c.Next()
	}
}
