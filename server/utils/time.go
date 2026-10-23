// Package utils 提供时间处理、时区偏移和日期计算功能
package utils

import (
	"math"
	"sync"
	"time"
)

// timeOffset 全局时间偏移量，用于模拟或时区调整
var timeOffset time.Duration

// mu 保护 timeOffset 的并发读写安全
var mu sync.RWMutex

// SetTimeOffset 设置全局时间偏移量
func SetTimeOffset(d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	timeOffset = d
}

// GetTimeOffset 获取当前时间偏移量
func GetTimeOffset() time.Duration {
	mu.RLock()
	defer mu.RUnlock()
	return timeOffset
}

// Now 返回当前时间（加上偏移量）
func Now() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return time.Now().Add(timeOffset)
}

// DateFormat 标准日期格式 "2006-01-02"
const DateFormat = "2006-01-02"

// ParseDate 按 "2006-01-02" 格式解析日期字符串
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateFormat, s)
}

// FirstDayOfMonth 获取指定日期所在月的第一天
func FirstDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// LastDayOfMonth 获取指定日期所在月的最后一天
func LastDayOfMonth(t time.Time) time.Time {
	return FirstDayOfMonth(t).AddDate(0, 1, -1)
}

// CalcProratedAmount 按天折算比例金额（计费公用口径，勿擅自修改）。
// 天数 = (end-start) 按整日取整 + 1，即起止两端日均计入（单日也要算 1 天）；
// 分母为调用方传入的自然月天数；结果按分四舍五入（math.Round）。start > end 返回 0。
func CalcProratedAmount(rentPrice float64, start, end time.Time, daysInMonth int) float64 {
	if start.After(end) {
		return 0
	}
	days := int(math.Round(end.Sub(start).Hours()/24)) + 1
	amount := rentPrice * float64(days) / float64(daysInMonth)
	return math.Round(amount*100) / 100
}

// MonthlyFinanceSummary 月度财务汇总，包含总收入、总支出和净利润
type MonthlyFinanceSummary struct {
	TotalIncome  float64
	TotalExpense float64
	NetProfit    float64
}

// NewMonthlyFinanceSummary 创建月度财务汇总并计算净利润
func NewMonthlyFinanceSummary(totalIncome, totalExpense float64) MonthlyFinanceSummary {
	return MonthlyFinanceSummary{
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		NetProfit:    totalIncome - totalExpense,
	}
}

// DynamicRoomStatus 根据数据库状态和到期日期计算房间动态状态。
//   - 非 rented 状态直接透传，不做推导；
//   - rented 今天到期或未来 30 天内（today <= end < today+30，恰好第30天不算）→ expiring；
//     end 已过 → expired；end 解析失败按 rented 处理；
//   - 剩余租期超过 30 天时保持 rented。
// 时区边界：today 截断到本地日零点，end 解析为 UTC 零点（time.Parse）。
// 两个绝对时刻比较：东八区本地日 D 的零点（UTC 前一天 16:00）恒早于 end（D 00:00 UTC），
// 因此到期日 D 一整天都判 expiring，直到次日本地 00:00 才翻转为 expired。
// （与 building.go IsVisible / auth.go IsBuildingExpired 用实时 Now() 的口径不同：
//   那两处因未截断，东八区在到期日当天本地 08:00 后就判已过期——各处边界不一致。）
func DynamicRoomStatus(dbStatus string, endDate string) string {
	if dbStatus != "rented" || endDate == "" {
		return dbStatus
	}
	end, err := ParseDate(endDate)
	if err != nil {
		return dbStatus
	}
	now := Now()
	// 截断到本地日期消除时分秒差异；截断后的 today 是本地日零点（绝对时刻），
	// 东八区下到期日当天全天不会判 expired，次日本地零点才翻转（见函数头注释）
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if today.After(end) {
		return "expired"
	}
	if end.Before(today.AddDate(0, 0, 30)) {
		return "expiring"
	}
	return dbStatus
}
