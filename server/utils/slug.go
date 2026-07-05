package utils

import (
	"regexp"
	"strings"

	"github.com/mozillazg/go-pinyin"
)

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
