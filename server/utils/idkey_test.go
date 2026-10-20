package utils

import "testing"

func TestParseIdKey(t *testing.T) {
	cases := []struct {
		in   string
		want uint
		ok   bool
	}{
		{"1", 1, true},
		{"123", 123, true},
		{"ygxy_1", 1, true},
		{"205_8", 8, true},
		{"a301_12", 12, true},
		{"multi_word_99", 99, true},
		{"abc", 0, false},
		{"ygxy_", 0, false},
		{"", 0, false},
		{"ygxy_x1", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseIdKey(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("ParseIdKey(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
