package utils

import "strings"

// RemarkMaxLen 备注类自由文本的统一长度上限（对应数据库 varchar(500) 列）。
const RemarkMaxLen = 500

// ClipText 去掉首尾空白，并按 rune 截断到 max 个字符。
//
// 用途：限制写入定长列（varchar(n)）的自由文本长度。
//   - MySQL 的 VARCHAR(n) 按字符计，超长会直接报 "Data too long for column"；
//   - SQLite 不限制列宽，不截断就会出现「写进去了、读出来被数据库悄悄截短」的假象。
//
// 按 rune 而不是字节截断，避免把一个中文字切成半个 UTF-8 序列（写库时直接报错）。
func ClipText(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}

// ClipRemark 备注类字段的统一入口：裁剪到 RemarkMaxLen。
func ClipRemark(s string) string {
	return ClipText(s, RemarkMaxLen)
}
