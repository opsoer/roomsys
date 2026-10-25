// 工具包，提供「查看房东完整电话」的按 IP 额度控制。
// 用途：公开接口不再返回完整号码，唯一出口是 reveal 接口；
// 每个 IP 在窗口内前 N 次直接放行（N 来自配置 server.reveal_free_per_day），
// 超过后进入「必须验证码」的粘性状态：验证码校验成功才解除，且计数减半。
package utils

import (
	"time"

	"rental-server/utils/ipwindow"
)

// revealQuotaWindow 免费额度的统计窗口。
// 用短窗口而非自然日，是为了让爬虫在免费期能拿到的数据量足够小：
// 一旦超过阈值立刻进入挑战状态，而不是等到当天结束。
const revealQuotaWindow = time.Minute

// revealQuotaKeepRatio 验证码校验成功后保留的计数百分比。
// 不清零：若验证成功即归零，爬虫可用 OCR 换「再来 N 次免费额度」，
// 成本固定且很低；减半后查看量越大，验证码次数越多，OCR 成本线性增长。
const revealQuotaKeepRatio = 50

// revealQuota 免费额度计数器。Stateful 开启「超限后粘性挑战」状态机。
// 调用方通过 InitRevealQuota 注入配置（免费次数来自 config.yaml）。
var revealQuota = ipwindow.New(ipwindow.Config{
	Limit:            5,
	Window:           revealQuotaWindow,
	Stateful:         true,
	ResolveKeepRatio: revealQuotaKeepRatio,
	MaxSizePerShard:  4096,
})

// InitRevealQuota 用配置中的免费次数重建计数器。仅在启动时调用一次；
// 必须在 HTTP 服务开始接收请求之前完成。limit <= 0 时回退到默认 5 次。
func InitRevealQuota(limit int) {
	if limit <= 0 {
		limit = 5
	}
	revealQuota = ipwindow.New(ipwindow.Config{
		Limit:            int64(limit),
		Window:           revealQuotaWindow,
		Stateful:         true,
		ResolveKeepRatio: revealQuotaKeepRatio,
		MaxSizePerShard:  4096,
	})
}

// RevealHit 记录一次查看请求并返回是否仍在免费额度内。
// 超出额度的同时会把该 IP 标记为「必须验证码」，之后的请求即使计数因
// 减半回落也会持续要求验证码，直到 MarkRevealVerified 被调用。
func RevealHit(ip string) (count int64, allowed bool) {
	return revealQuota.Hit(ip)
}

// RevealNeedsCaptcha 返回该 IP 当前是否处于「必须验证码」状态。
func RevealNeedsCaptcha(ip string) bool {
	return revealQuota.State(ip) == ipwindow.StateChallenged
}

// RevealCount 返回 IP 在当前窗口内已记录的查看次数。
func RevealCount(ip string) int64 {
	return revealQuota.Count(ip)
}

// MarkRevealVerified 在验证码校验成功后调用：解除挑战状态，计数减半。
// 返回减半后保留的计数。
func MarkRevealVerified(ip string) int64 {
	return revealQuota.Resolve(ip)
}

// MarkRevealChallenged 把 IP 标记为「必须验证码」，用于校验失败或运营处置。
func MarkRevealChallenged(ip string) {
	revealQuota.Challenged(ip)
}

// ResetReveal 清除 IP 的额度记录（运营手动解封）。
func ResetReveal(ip string) {
	revealQuota.Reset(ip)
}
