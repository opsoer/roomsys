package utils

import (
	"math"
	"testing"
	"time"
)

// round2 保留两位小数（与金额计算口径一致）。
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("解析日期失败: %v", err)
	}
	return d
}

// 分摊计算：按天折算租金与管理费。
// 口径（见 CalcProratedAmount）：天数 = end - start + 1（含首尾），金额 = 月租 × 天数 / 当月天数。
func TestCalcProratedAmount(t *testing.T) {
	cases := []struct {
		name        string
		rent        float64
		start       string
		end         string
		daysInMonth int
		want        float64
	}{
		{"整月", 2000, "2026-10-01", "2026-10-31", 31, 2000},
		{"月中起租半个月", 2000, "2026-10-16", "2026-10-31", 31, round2(2000 * 16.0 / 31.0)},
		{"月初退租半个月", 2000, "2026-10-01", "2026-10-16", 31, round2(2000 * 16.0 / 31.0)},
		{"跨月按传入区间天数", 2000, "2026-09-20", "2026-10-10", 31, round2(2000 * 21.0 / 31.0)},
		{"金额为0", 0, "2026-10-01", "2026-10-31", 31, 0},
		{"起止颠倒返回0", 2000, "2026-10-20", "2026-10-10", 31, 0},
	}
	for _, tc := range cases {
		got := CalcProratedAmount(tc.rent, mustDate(t, tc.start), mustDate(t, tc.end), tc.daysInMonth)
		if diff := got - tc.want; diff > 0.01 || diff < -0.01 {
			t.Errorf("%s: 期望 %.2f，实际 %.2f", tc.name, tc.want, got)
		}
	}
}

// 房间动态状态：结合合同到期日推导对外可见状态。
// 口径：已过期 → expired；30 天内到期（含今天）→ expiring；否则保持原状态。
func TestDynamicRoomStatus(t *testing.T) {
	now := Now()
	today := now.Format("2006-01-02")
	past := now.AddDate(0, 0, -5).Format("2006-01-02")
	soon := now.AddDate(0, 0, 10).Format("2006-01-02")
	far := now.AddDate(0, 0, 60).Format("2006-01-02")

	cases := []struct {
		dbStatus string
		endDate  string
		want     string
	}{
		{"vacant", "", "vacant"},
		{"rented", far, "rented"},
		{"rented", soon, "expiring"},
		{"rented", past, "expired"},
		{"rented", today, "expiring"},
		{"reserved", "", "reserved"},
		{"rented", "非法日期", "rented"},
	}
	for _, tc := range cases {
		if got := DynamicRoomStatus(tc.dbStatus, tc.endDate); got != tc.want {
			t.Errorf("状态(%s,%s): 期望 %s，实际 %s", tc.dbStatus, tc.endDate, tc.want, got)
		}
	}
}

// 月初/月末。
func TestFirstAndLastDayOfMonth(t *testing.T) {
	now := mustDate(t, "2026-02-15")
	if got := FirstDayOfMonth(now).Format("2006-01-02"); got != "2026-02-01" {
		t.Errorf("月初应为 2026-02-01，实际 %s", got)
	}
	if got := LastDayOfMonth(now).Format("2006-01-02"); got != "2026-02-28" {
		t.Errorf("月末应为 2026-02-28，实际 %s", got)
	}
	// 闰年
	leap := mustDate(t, "2028-02-10")
	if got := LastDayOfMonth(leap).Format("2006-01-02"); got != "2028-02-29" {
		t.Errorf("闰年月末应为 2028-02-29，实际 %s", got)
	}
}

// 月度财务汇总。
func TestNewMonthlyFinanceSummary(t *testing.T) {
	s := NewMonthlyFinanceSummary(10000, 3000)
	if s.TotalIncome != 10000 || s.TotalExpense != 3000 || s.NetProfit != 7000 {
		t.Fatalf("汇总不符: %+v", s)
	}
	// 支出大于收入时净利为负
	s2 := NewMonthlyFinanceSummary(1000, 3000)
	if s2.NetProfit != -2000 {
		t.Fatalf("净利应为 -2000，实际 %v", s2.NetProfit)
	}
}

// 日期解析。
func TestParseDate(t *testing.T) {
	if _, err := ParseDate("2026-10-05"); err != nil {
		t.Fatalf("合法日期应解析成功: %v", err)
	}
	if _, err := ParseDate("2026/10/05"); err == nil {
		t.Fatal("非法日期应返回错误")
	}
	if _, err := ParseDate(""); err == nil {
		t.Fatal("空字符串应返回错误")
	}
}

// 手机号脱敏：13812345678 → 138****5678；已含 * 原样返回（幂等）；长度不足 7 原样返回。
func TestMaskPhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"13812345678", "138****5678"},
		{"1381234", "138****1234"},
		{"138****5678", "138****5678"},
		{"", ""},
		{"123", "123"},
	}
	for _, tc := range cases {
		if got := MaskPhone(tc.in); got != tc.want {
			t.Errorf("脱敏 %q: 期望 %q，实际 %q", tc.in, tc.want, got)
		}
	}
}

// slug 生成：中文转拼音 slug（小写 + 连字符）；无中文时退化为 ASCII slug；全无效字符时 unnamed。
func TestSlugify(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"深圳  好租", "shen-zhen-hao-zu"},
		{"a/b\\c", "a-b-c"},
		{"Hello 123", "hello-123"},
		{"", "unnamed"},
		{"!!!", "unnamed"},
	}
	for _, tc := range cases {
		if got := Slugify(tc.in); got != tc.want {
			t.Errorf("Slugify(%q): 期望 %q，实际 %q", tc.in, tc.want, got)
		}
	}
}

// 模拟时间偏移：设置后 Now() 应随之偏移，且可重置。
func TestTimeOffset(t *testing.T) {
	origin := Now()
	SetTimeOffset(24 * time.Hour)
	if diff := Now().Sub(origin); diff < 23*time.Hour || diff > 25*time.Hour {
		t.Fatalf("偏移未生效: %v", diff)
	}
	SetTimeOffset(0)
	if diff := Now().Sub(origin); diff < -time.Minute || diff > time.Minute {
		t.Fatalf("偏移未重置: %v", diff)
	}
}
