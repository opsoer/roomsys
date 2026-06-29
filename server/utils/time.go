package utils

import (
	"fmt"
	"sync"
	"time"
)

var (
	billNoMu sync.Mutex
	billNoSeq int64
)

func GenerateBillNo() string {
	billNoMu.Lock()
	defer billNoMu.Unlock()
	billNoSeq++
	ts := Now().Format("20060102150405")
	return fmt.Sprintf("B%s%04d", ts, billNoSeq%10000)
}

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
