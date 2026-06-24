package utils

import (
	"sync"
	"time"
)

var (
	timeOffset time.Duration
	mu         sync.RWMutex
)

func SetTimeOffset(d time.Duration) {
	mu.Lock()
	defer mu.Unlock()
	timeOffset = d
}

func GetTimeOffset() time.Duration {
	mu.RLock()
	defer mu.RUnlock()
	return timeOffset
}

func Now() time.Time {
	mu.RLock()
	defer mu.RUnlock()
	return time.Now().Add(timeOffset)
}

func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}
