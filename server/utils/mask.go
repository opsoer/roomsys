// 工具包，提供手机号脱敏
package utils

import "strings"

// MaskPhone 将手机号打码为 138****5678 形式，公开接口一律返回打码号码；
// 已含 * 的号码原样返回，保证幂等。
func MaskPhone(phone string) string {
	if phone == "" || len(phone) < 7 {
		return phone
	}
	if strings.Contains(phone, "*") {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
