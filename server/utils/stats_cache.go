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

// CacheGetOrSet 从缓存获取未过期数据，未命中（或已过期）则调用 fetch 生成并写回。
// 注意：无并发去重（无 singleflight）——缓存过期的瞬间可能有多个请求各自执行 fetch，
// 后写覆盖先写；fetch 出错时直接透传错误，不会回退使用已过期的旧值。
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

// ClearStatsCache 清空统计数据缓存。
// 供测试或运维在数据切换后强制重新计算；生产请求路径不应调用。
func ClearStatsCache() {
	statsCache.Range(func(key, _ interface{}) bool {
		statsCache.Delete(key)
		return true
	})
}
