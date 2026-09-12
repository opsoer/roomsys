// 工具包，提供统计数据缓存功能
package utils

import (
	"sync"
	"time"
)

// cacheEntry 缓存条目，包含数据和过期时间
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// statsCache 统计数据缓存，使用 sync.Map 保证并发安全
var statsCache sync.Map

// CacheGetOrSet 从缓存获取数据，不存在则调用 fetch 生成并缓存
func CacheGetOrSet(key string, ttl time.Duration, fetch func() (interface{}, error)) (interface{}, error) {
	if val, ok := statsCache.Load(key); ok {
		entry := val.(*cacheEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.data, nil
		}
	}
	data, err := fetch()
	if err != nil {
		return nil, err
	}
	statsCache.Store(key, &cacheEntry{
		data:      data,
		expiresAt: time.Now().Add(ttl),
	})
	return data, nil
}
