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

// Until 计算距目标时间的剩余时长
func Until(t time.Time) time.Duration {
	return t.Sub(Now())
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

// MonthStr 返回 "2006-01" 格式的月份字符串
func MonthStr(t time.Time) string {
	return t.Format("2006-01")
}

// MonthBoundary 月份起止边界，包含首日、末日和月份字符串
type MonthBoundary struct {
	FirstDay time.Time
	LastDay  time.Time
	Month    string
}

// GetMonthBoundary 获取指定日期所在月份的起止边界
func GetMonthBoundary(t time.Time) MonthBoundary {
	first := FirstDayOfMonth(t)
	return MonthBoundary{
		FirstDay: first,
		LastDay:  LastDayOfMonth(t),
		Month:    MonthStr(t),
	}
}

// CalcProratedAmount 按天计算按比例分摊的租金金额
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

// DynamicRoomStatus 根据数据库状态和到期日期计算房间动态状态（正常/即将到期/已过期）
func DynamicRoomStatus(dbStatus string, endDate string) string {
	if dbStatus != "rented" || endDate == "" {
		return dbStatus
	}
	end, err := ParseDate(endDate)
	if err != nil {
		return dbStatus
	}
	now := Now()
	if now.After(end) {
		return "expired"
	}
	if end.Before(now.AddDate(0, 0, 30)) {
		return "expiring"
	}
	return dbStatus
}
