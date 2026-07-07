package utils

import (
	"sync"
	"time"
)

type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

var statsCache sync.Map

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

func CacheInvalidate(key string) {
	statsCache.Delete(key)
}

func CacheInvalidateAll() {
	statsCache.Range(func(key, _ interface{}) bool {
		statsCache.Delete(key)
		return true
	})
}
