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
// 适用场景：统计数据被整体性变更后强制重新计算——包括测试隔离（testkit 每个用例
// 清一次防串味）与生产中删除公寓（handlers/buildings.go 删除级联清掉该公寓的
// 浏览数据后立即失效缓存，避免 5 分钟 TTL 内数据面板仍显示已删公寓的排行）。
func ClearStatsCache() {
	statsCache.Range(func(key, _ interface{}) bool {
		statsCache.Delete(key)
		return true
	})
	// 存活楼栋缓存也属于统计写入链路，测试隔离/删除公寓时一并失效。
	ClearBuildingAliveCache()
}
