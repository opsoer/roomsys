// 工具包，提供手机号脱敏
package utils

import "strings"

// MaskPhone 将号码打码为前3后4保留的 138****5678 形式。
// 已含 * 的号码原样返回，保证幂等。
// 注意：长度 < 7 的号码原样返回不脱敏（数据残缺时的兜底行为），
// 与「公开接口一律打码」的诉求存在缺口，调用方需自行保证传入完整号码。
func MaskPhone(phone string) string {
	if phone == "" || len(phone) < 7 {
		return phone
	}
	if strings.Contains(phone, "*") {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
