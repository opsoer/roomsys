// 私享模式（private）下的公开接口守卫：试用/独立部署时，
// 公寓列表、位置聚合与申请入驻等「平台级」公开接口仅超级管理员可用，
// 防止有人绕过前端直接调 API 拿到全平台公寓清单；公寓详情类接口不受影响，
// 访客仍可通过链接/二维码直达单个公寓页。
package middleware

import (
	"net/http"
	"strings"

	"rental-server/config"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
)

// PrivateGuard 返回私享模式守卫：mode 非 private 时直接放行（平台行为不变）；
// private 模式下要求请求携带有效的超级管理员令牌，否则按 404 处理（不暴露接口存在性）。
func PrivateGuard(mode, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if mode != config.ModePrivate {
			c.Next()
			return
		}
		if isSuperAdminRequest(c, jwtSecret) {
			c.Next()
			return
		}
		utils.Error(c, http.StatusNotFound, "接口不存在")
		c.Abort()
	}
}

// isSuperAdminRequest 从 Authorization: Bearer <token> 解析令牌并判断是否超级管理员。
// 公开路由没有 AuthMiddleware 前置，这里自行解析；无效/缺失令牌一律视为非管理员。
func isSuperAdminRequest(c *gin.Context, secret string) bool {
	if secret == "" {
		return false
	}
	auth := c.GetHeader("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return false
	}
	claims, err := utils.ParseToken(strings.TrimPrefix(auth, "Bearer "), secret)
	if err != nil || claims == nil {
		return false
	}
	return claims.Role == "super_admin"
}
