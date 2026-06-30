package utils

import (
	"fmt"
	"math"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	billNoMu   sync.Mutex
	billNoSeq  int64
	billNoOnce sync.Once
	billNoDB   *gorm.DB
)

func InitBillNo(db *gorm.DB) {
	billNoDB = db
	loadBillNoSeq()
}

func loadBillNoSeq() {
	if billNoDB == nil {
		return
	}
	type result struct {
		MaxBillNo string
	}
	var r result
	billNoDB.Raw("SELECT MAX(bill_no) AS max_bill_no FROM bills").Scan(&r)
	if r.MaxBillNo != "" && len(r.MaxBillNo) > 15 {
		seqStr := r.MaxBillNo[15:]
		fmt.Sscanf(seqStr, "%04d", &billNoSeq)
	}
}

func GenerateBillNo() string {
	billNoMu.Lock()
	defer billNoMu.Unlock()
	billNoSeq++
	ts := Now().Format("20060102150405")
	return fmt.Sprintf("B%s%04d", ts, billNoSeq%10000)
}

var (
	timeOffset time.Duration
	mu         sync.RWMutex
)

func SetTimeOffset(d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	timeOffset = d
}

func GetTimeOffset() time.Duration {
	mu.RLock()
	defer mu.RUnlock()
	return timeOffset
}

func Now() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return time.Now().Add(timeOffset)
}

func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}

const DateFormat = "2006-01-02"

func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateFormat, s)
}

func FirstDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func LastDayOfMonth(t time.Time) time.Time {
	return FirstDayOfMonth(t).AddDate(0, 1, -1)
}

func MonthStr(t time.Time) string {
	return t.Format("2006-01")
}

type MonthBoundary struct {
	FirstDay time.Time
	LastDay  time.Time
	Month    string
}

func GetMonthBoundary(t time.Time) MonthBoundary {
	first := FirstDayOfMonth(t)
	return MonthBoundary{
		FirstDay: first,
		LastDay:  LastDayOfMonth(t),
		Month:    MonthStr(t),
	}
}

func CalcProratedAmount(rentPrice float64, start, end time.Time, daysInMonth int) float64 {
	if start.After(end) {
		return 0
	}
	days := int(math.Round(end.Sub(start).Hours()/24)) + 1
	amount := rentPrice * float64(days) / float64(daysInMonth)
	return math.Round(amount*100) / 100
}

type MonthlyFinanceSummary struct {
	TotalIncome  float64
	TotalExpense float64
	NetProfit    float64
}

func NewMonthlyFinanceSummary(totalIncome, totalExpense float64) MonthlyFinanceSummary {
	return MonthlyFinanceSummary{
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		NetProfit:    totalIncome - totalExpense,
	}
}
