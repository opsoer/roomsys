package utils

import (
	"testing"
	"time"
)

func TestGenerateBillNo(t *testing.T) {
	billNoSeq = 0
	no1 := GenerateBillNo()
	no2 := GenerateBillNo()

	if len(no1) != 20 {
		t.Errorf("BillNo length = %d, want 20", len(no1))
	}
	if no1[:1] != "B" {
		t.Errorf("BillNo prefix = %s, want B", no1[:1])
	}
	if no1 == no2 {
		t.Errorf("BillNos should be unique, got %s == %s", no1, no2)
	}
}

func TestTimeOffset(t *testing.T) {
	SetTimeOffset(0)
	if GetTimeOffset() != 0 {
		t.Errorf("offset should be 0")
	}

	SetTimeOffset(1 * time.Hour)
	if GetTimeOffset() != 1*time.Hour {
		t.Errorf("offset should be 1h")
	}

	now := Now()
	expected := time.Now().Add(1 * time.Hour)
	if now.Sub(expected) > 2*time.Second {
		t.Errorf("Now() should reflect offset")
	}

	SetTimeOffset(0)
}

func TestFirstDayOfMonth(t *testing.T) {
	date := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	first := FirstDayOfMonth(date)
	if first.Day() != 1 {
		t.Errorf("FirstDay = %d, want 1", first.Day())
	}
	if first.Month() != 3 {
		t.Errorf("Month = %d, want 3", first.Month())
	}
}

func TestLastDayOfMonth(t *testing.T) {
	date := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	last := LastDayOfMonth(date)
	if last.Day() != 28 {
		t.Errorf("LastDay of Feb 2026 = %d, want 28", last.Day())
	}

	date31 := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	last31 := LastDayOfMonth(date31)
	if last31.Day() != 31 {
		t.Errorf("LastDay of Jan 2026 = %d, want 31", last31.Day())
	}
}

func TestMonthStr(t *testing.T) {
	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	if MonthStr(date) != "2026-03" {
		t.Errorf("MonthStr = %s, want 2026-03", MonthStr(date))
	}
}

func TestParseDate(t *testing.T) {
	d, err := ParseDate("2026-03-15")
	if err != nil {
		t.Errorf("ParseDate error: %v", err)
	}
	if d.Day() != 15 || d.Month() != 3 || d.Year() != 2026 {
		t.Errorf("ParseDate result = %v, want 2026-03-15", d)
	}
}

func TestCalcProratedAmount(t *testing.T) {
	start := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	amount := CalcProratedAmount(3000, start, end, 31)

	expected := 3000.0 * 10.0 / 31.0
	if diff := amount - expected; diff > 0.01 || diff < -0.01 {
		t.Errorf("CalcProratedAmount = %.2f, want %.2f", amount, expected)
	}
}

func TestCalcProratedAmount_StartAfterEnd(t *testing.T) {
	start := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	amount := CalcProratedAmount(3000, start, end, 31)
	if amount != 0 {
		t.Errorf("CalcProratedAmount with start>end = %.2f, want 0", amount)
	}
}

func TestNewMonthlyFinanceSummary(t *testing.T) {
	s := NewMonthlyFinanceSummary(10000, 6000)
	if s.TotalIncome != 10000 {
		t.Errorf("TotalIncome = %.2f, want 10000", s.TotalIncome)
	}
	if s.TotalExpense != 6000 {
		t.Errorf("TotalExpense = %.2f, want 6000", s.TotalExpense)
	}
	if s.NetProfit != 4000 {
		t.Errorf("NetProfit = %.2f, want 4000", s.NetProfit)
	}
}

func TestGetMonthBoundary(t *testing.T) {
	date := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	b := GetMonthBoundary(date)
	if b.Month != "2026-03" {
		t.Errorf("Month = %s, want 2026-03", b.Month)
	}
	if b.FirstDay.Day() != 1 {
		t.Errorf("FirstDay = %d, want 1", b.FirstDay.Day())
	}
	if b.LastDay.Day() != 31 {
		t.Errorf("LastDay = %d, want 31", b.LastDay.Day())
	}
}
