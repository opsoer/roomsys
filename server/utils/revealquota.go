// 工具包，提供「查看房东完整电话」的按 IP 每日计数。
// 用途：公开接口不再返回完整号码，唯一出口是 reveal 接口；
// 每个 IP 每日前 N 次直接放行（N 来自配置 reveal_free_per_day），之后每次要求图片验证码。
package utils

import (
	"sync"
	"time"
)

// revealQuotaEntry 单 IP 的当日查看记录
type revealQuotaEntry struct {
	day   string // 自然日标识，跨日自动清零
	count int
}

var (
	revealQuotaMu     sync.Mutex
	revealQuotaCounts = make(map[string]*revealQuotaEntry)
)

// todayKey 返回本地时区的自然日标识。
func todayKey() string { return Now().Format("2006-01-02") }

// RevealCount 返回 IP 当日已成功查看完整电话的次数，跨日自动归零。
func RevealCount(ip string) int {
	revealQuotaMu.Lock()
	defer revealQuotaMu.Unlock()
	e, ok := revealQuotaCounts[ip]
	if !ok || e.day != todayKey() {
		return 0
	}
	return e.count
}

// IncrReveal 累加 IP 当日查看次数并返回最新值。
func IncrReveal(ip string) int {
	revealQuotaMu.Lock()
	defer revealQuotaMu.Unlock()
	today := todayKey()
	e, ok := revealQuotaCounts[ip]
	if !ok || e.day != today {
		e = &revealQuotaEntry{day: today}
		revealQuotaCounts[ip] = e
	}
	e.count++
	return e.count
}

// init 启动每小时清理协程，丢弃跨日的旧记录，防止 map 无限增长。
func init() {
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			today := todayKey()
			revealQuotaMu.Lock()
			for ip, e := range revealQuotaCounts {
				if e.day != today {
					delete(revealQuotaCounts, ip)
				}
			}
			revealQuotaMu.Unlock()
		}
	}()
}
