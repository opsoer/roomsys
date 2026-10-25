// handlers 包，提供图片验证码的生成与校验。
// 用于「查看房东完整电话」的人机校验：同一 IP 在额度窗口内免费查看次数用完后，
// reveal 接口要求提交这里生成的验证码，通过后放行（一次性使用）。
package handlers

import (
	"strconv"
	"time"

	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/mojocn/base64Captcha"
)

// 验证码参数：4 位数字、答案存内存 5 分钟过期，验证通过即销毁。
//
// 图片尺寸的选择依据（base64Captcha 内部按 11×18 的数字字模等比缩放）：
//   - 240×80（旧值）：数字高 40px、宽 24px、描边 2px，面积 19200
//   - 180×70（现值）：数字高 36px、宽 22px、描边 2px，面积 12600（-34%）
//     数字仅比旧值小 10%，2px 描边保留，手机端与缩放后仍可辨认；
//   - 160×64：数字高 32px 但描边降到 1px，小屏或缩放时容易糊，不采用。
//
// 扭曲幅度与噪点数量同步下调，避免小图上噪点挡住笔画：
// 扭曲由 distort 内部固定为 5~10 像素幅度，线宽随图片缩小自然变细；
// 背景圆点从 60 降到 30，既保持防 OCR 干扰，又不在 180×70 的图上糊成一片。
const (
	captchaImageWidth  = 180
	captchaImageHeight = 70
	captchaDigits      = 4
	captchaSkew        = 0.35
	captchaDotCount    = 30
	captchaTTL         = 5 * time.Minute
	captchaStoreSize   = 10240
)

var (
	captchaStore = base64Captcha.NewMemoryStore(captchaStoreSize, captchaTTL)
	captchaGen   = base64Captcha.NewCaptcha(
		base64Captcha.NewDriverDigit(captchaImageHeight, captchaImageWidth, captchaDigits, captchaSkew, captchaDotCount),
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

// CaptchaImageSize 返回验证码图片尺寸，供前端或测试断言使用。
func CaptchaImageSize() (width, height int) {
	return captchaImageWidth, captchaImageHeight
}

// CaptchaDebugInfo 返回验证码参数摘要，便于排查「图片看不清」的反馈。
func CaptchaDebugInfo() string {
	return "digits=" + strconv.Itoa(captchaDigits) +
		" size=" + strconv.Itoa(captchaImageWidth) + "x" + strconv.Itoa(captchaImageHeight) +
		" skew=" + strconv.FormatFloat(captchaSkew, 'f', 2, 64) +
		" dots=" + strconv.Itoa(captchaDotCount)
}
