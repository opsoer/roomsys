// handlers 包，提供图片验证码的生成与校验。
// 用于「查看房东完整电话」的人机校验：每个 IP 每日免费次数用完后，
// reveal 接口要求提交这里生成的验证码，通过后放行（一次性使用）。
package handlers

import (
	"time"

	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
)

// 验证码实例：4 位数字、答案存内存 5 分钟过期，验证通过即销毁。
var (
	captchaStore = base64Captcha.NewMemoryStore(10240, 5*time.Minute)
	captchaGen   = base64Captcha.NewCaptcha(
		base64Captcha.NewDriverDigit(80, 240, 4, 0.45, 60),
		captchaStore,
	)
)

// GetCaptcha 生成一张图片验证码，返回验证码 ID 与 base64 图片。
func GetCaptcha(c *gin.Context) {
	id, b64, _, err := captchaGen.Generate()
	if err != nil {
		utils.Error(c, 500, "验证码生成失败")
		return
	}
	utils.Success(c, gin.H{"captcha_id": id, "image": b64})
}

// VerifyCaptcha 校验用户提交的验证码；无论对错都销毁该验证码，防止重放。
func VerifyCaptcha(id, code string) bool {
	if id == "" || code == "" {
		return false
	}
	return captchaStore.Verify(id, code, true)
}
