package utils

import (
	"strconv"
	"strings"
)

// ParseIdKey 解析公开页 URL 中的数字标识：支持纯数字（"1"）与「缩写_id」（"ygxy_1"、"205_8"）两种形式。
// 缩写部分仅用于提升链接可读性（拼音首字母/房号），解析只取最后一个下划线之后的数字，
// 因此前缀缺失或与当前名称不一致都不影响访问。
func ParseIdKey(s string) (uint, bool) {
	if i := strings.LastIndexByte(s, '_'); i >= 0 {
		s = s[i+1:]
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return uint(n), true
}
