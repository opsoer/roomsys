// 工具包，提供字符串转拼音 Slug 功能
package utils

import (
	"regexp"
	"strings"

	"github.com/mozillazg/go-pinyin"
)

// Slugify 将中文字符串转换为拼音 slug（小写字母 + 连字符）
func Slugify(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unnamed"
	}

	args := pinyin.NewArgs()
	args.Style = pinyin.Normal
	segments := pinyin.Pinyin(s, args)

	var parts []string
	for _, seg := range segments {
		for _, p := range seg {
			if p != "" {
				parts = append(parts, p)
			}
		}
	}

	slug := strings.Join(parts, "-")
	if slug == "" {
		// 没有中文可转写（如纯英文/数字名称）时，退化为原字符串的 ASCII slug，
		// 避免所有同类名称都变成 "unnamed" 而撞到同一目录。
		slug = s
	}
	re := regexp.MustCompile(`[^a-zA-Z0-9-]+`)
	slug = re.ReplaceAllString(slug, "-")
	re2 := regexp.MustCompile(`-+`)
	slug = re2.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	slug = strings.ToLower(slug)

	if slug == "" {
		return "unnamed"
	}
	return slug
}
