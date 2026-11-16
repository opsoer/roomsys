package utils

import (
	"strings"
	"testing"
)

func TestClipText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"空串", "", 500, ""},
		{"未超长", "粤B12345", 500, "粤B12345"},
		{"恰好等长", strings.Repeat("备", 500), 500, strings.Repeat("备", 500)},
		{"超出按字符截断", strings.Repeat("备", 600), 500, strings.Repeat("备", 500)},
		{"去首尾空白", "  有车  ", 500, "有车"},
		{"按字符而非字节截断", strings.Repeat("备", 499) + "x", 500, strings.Repeat("备", 499) + "x"},
		{"max<=0 一律空", "abc", 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ClipText(c.in, c.max); got != c.want {
				t.Fatalf("ClipText(%q,%d) = %q，期望 %q", c.in, c.max, got, c.want)
			}
		})
	}
}

func TestClipRemark(t *testing.T) {
	long := strings.Repeat("注", 1000)
	if got := ClipRemark(long); len([]rune(got)) != RemarkMaxLen {
		t.Fatalf("ClipRemark 长度 = %d，期望 %d", len([]rune(got)), RemarkMaxLen)
	}
}
