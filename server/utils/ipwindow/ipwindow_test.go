// ipwindow 包的单元测试：覆盖窗口边界、并发计数、状态机、清理与容量淘汰。
package ipwindow

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock 是可手动推进的测试时钟。
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 25, 12, 0, 0, 0, time.Local)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestHitWithinLimit(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 3, Window: time.Minute, Now: clock.Now})
	defer w.Close()

	for i := 1; i <= 3; i++ {
		count, allowed := w.Hit("1.1.1.1")
		if count != int64(i) || !allowed {
			t.Fatalf("第 %d 次：count=%d allowed=%v，期望 count=%d allowed=true", i, count, allowed, i)
		}
	}
	count, allowed := w.Hit("1.1.1.1")
	if count != 4 || allowed {
		t.Fatalf("第 4 次：count=%d allowed=%v，期望 count=4 allowed=false", count, allowed)
	}
	if got := w.Count("1.1.1.1"); got != 4 {
		t.Fatalf("Count 期望 4，实际 %d", got)
	}
	if got := w.Count("2.2.2.2"); got != 0 {
		t.Fatalf("其他 IP Count 期望 0，实际 %d", got)
	}
}

func TestWindowReset(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 2, Window: time.Minute, Now: clock.Now})
	defer w.Close()

	for i := 0; i < 3; i++ {
		w.Hit("1.1.1.1")
	}
	if _, allowed := w.Hit("1.1.1.1"); allowed {
		t.Fatal("超限后应被拒绝")
	}

	clock.Advance(time.Minute)
	count, allowed := w.Hit("1.1.1.1")
	if count != 1 || !allowed {
		t.Fatalf("跨窗口后应重新计数：count=%d allowed=%v，期望 count=1 allowed=true", count, allowed)
	}
}

func TestWindowBoundaryExact(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 1, Window: time.Second, Now: clock.Now})
	defer w.Close()

	if _, allowed := w.Hit("9.9.9.9"); !allowed {
		t.Fatal("窗口内第一次应放行")
	}
	clock.Advance(time.Second)
	if _, allowed := w.Hit("9.9.9.9"); !allowed {
		t.Fatal("恰好到达窗口边界时应重新计数并放行")
	}
}

func TestConcurrentHitNeverOverLimit(t *testing.T) {
	w := New(Config{Limit: 100, Window: time.Minute})
	defer w.Close()

	const goroutines = 200
	var allowedCount atomic.Int64
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, allowed := w.Hit("10.0.0.1"); allowed {
				allowedCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if got := allowedCount.Load(); got != 100 {
		t.Fatalf("200 个并发请求 Limit=100，放行数应为 100，实际 %d", got)
	}
	if got := w.Count("10.0.0.1"); got != goroutines {
		t.Fatalf("计数应为 %d，实际 %d", goroutines, got)
	}
}

func TestConcurrentDifferentIPs(t *testing.T) {
	w := New(Config{Limit: 10, Window: time.Minute})
	defer w.Close()

	const ips = 1000
	var wg sync.WaitGroup
	wg.Add(ips)
	for i := 0; i < ips; i++ {
		go func(n int) {
			defer wg.Done()
			ip := fmt.Sprintf("10.0.%d.%d", n/256, n%256)
			for j := 0; j < 5; j++ {
				if _, allowed := w.Hit(ip); !allowed {
					t.Errorf("%s 前 5 次都应放行", ip)
				}
			}
		}(i)
	}
	wg.Wait()

	if got := w.Len(); got != ips {
		t.Fatalf("记录数应为 %d，实际 %d", ips, got)
	}
}

func TestStatefulChallengeIsSticky(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 5, Window: time.Minute, Stateful: true, Now: clock.Now})
	defer w.Close()

	for i := 1; i <= 5; i++ {
		if _, allowed := w.Hit("3.3.3.3"); !allowed {
			t.Fatalf("免费额度内第 %d 次应放行", i)
		}
	}
	if got := w.State("3.3.3.3"); got != StateNormal {
		t.Fatalf("额度内状态应为 StateNormal，实际 %v", got)
	}

	if _, allowed := w.Hit("3.3.3.3"); allowed {
		t.Fatal("超过额度应拒绝")
	}
	if got := w.State("3.3.3.3"); got != StateChallenged {
		t.Fatalf("超限后状态应为 StateChallenged，实际 %v", got)
	}

	// 挑战状态不允许随时间自动解除：即使窗口过期，仍然需要验证码
	clock.Advance(10 * time.Minute)
	if _, allowed := w.Hit("3.3.3.3"); allowed {
		t.Fatal("挑战状态是粘性的，窗口过期也不应放行")
	}
	if got := w.State("3.3.3.3"); got != StateChallenged {
		t.Fatalf("窗口过期后状态仍应为 StateChallenged，实际 %v", got)
	}
}

func TestStatefulResolveHalvesCount(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 5, Window: time.Minute, Stateful: true, ResolveKeepRatio: 50, Now: clock.Now})
	defer w.Close()

	for i := 1; i <= 6; i++ {
		w.Hit("4.4.4.4")
	}
	if got := w.State("4.4.4.4"); got != StateChallenged {
		t.Fatalf("应为 StateChallenged，实际 %v", got)
	}

	kept := w.Resolve("4.4.4.4")
	if kept != 3 {
		t.Fatalf("6 次减半应保留 3，实际 %d", kept)
	}
	if got := w.State("4.4.4.4"); got != StateNormal {
		t.Fatalf("验证成功后应回到 StateNormal，实际 %v", got)
	}

	// 保留 3 次，说明还能再用 2 次免费额度，第 3 次（总数第 6 次）再次触发挑战
	if _, allowed := w.Hit("4.4.4.4"); !allowed {
		t.Fatal("第 4 次（保留后第 1 次）应放行")
	}
	if _, allowed := w.Hit("4.4.4.4"); !allowed {
		t.Fatal("第 5 次应放行")
	}
	if _, allowed := w.Hit("4.4.4.4"); allowed {
		t.Fatal("第 6 次应再次触发挑战")
	}
}

func TestStatefulResolveKeepRatio100(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 1, Window: time.Minute, Stateful: true, ResolveKeepRatio: 100, Now: clock.Now})
	defer w.Close()

	w.Hit("5.5.5.5")
	w.Hit("5.5.5.5")
	if kept := w.Resolve("5.5.5.5"); kept != 2 {
		t.Fatalf("KeepRatio=100 应保留 2，实际 %d", kept)
	}
	// 计数未减，下一次立刻再次超限
	if _, allowed := w.Hit("5.5.5.5"); allowed {
		t.Fatal("KeepRatio=100 时验证成功也不应再放行")
	}
}

func TestChallengedMarksEntry(t *testing.T) {
	w := New(Config{Limit: 100, Window: time.Minute, Stateful: true})
	defer w.Close()

	w.Challenged("6.6.6.6")
	if got := w.State("6.6.6.6"); got != StateChallenged {
		t.Fatalf("应被标记为 StateChallenged，实际 %v", got)
	}
	if _, allowed := w.Hit("6.6.6.6"); allowed {
		t.Fatal("标记挑战后不应放行")
	}
}

func TestNonStatefulIgnoresState(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 2, Window: time.Minute, Now: clock.Now})
	defer w.Close()

	w.Challenged("7.7.7.7") // 非 Stateful 模式下应为空操作
	if got := w.State("7.7.7.7"); got != StateNormal {
		t.Fatalf("非 Stateful 模式状态应恒为 StateNormal，实际 %v", got)
	}
	if _, allowed := w.Hit("7.7.7.7"); !allowed {
		t.Fatal("非 Stateful 模式不应受挑战状态影响")
	}
}

func TestReset(t *testing.T) {
	w := New(Config{Limit: 1, Window: time.Minute})
	defer w.Close()

	w.Hit("8.8.8.8")
	if got := w.Count("8.8.8.8"); got != 1 {
		t.Fatalf("Count 期望 1，实际 %d", got)
	}
	w.Reset("8.8.8.8")
	if got := w.Count("8.8.8.8"); got != 0 {
		t.Fatalf("Reset 后 Count 期望 0，实际 %d", got)
	}
	if got := w.Len(); got != 0 {
		t.Fatalf("Reset 后 Len 期望 0，实际 %d", got)
	}
}

func TestEvictionOnCapacity(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: -1, Window: time.Minute, MaxSizePerShard: 8, Now: clock.Now})
	defer w.Close()

	// 每个桶上限 8，持续写入远超上限的 IP，总数必须收敛在上限内
	total := ShardCount * 8
	for i := 0; i < total*4; i++ {
		w.Hit(fmt.Sprintf("20.%d.%d.%d", i/65536, (i/256)%256, i%256))
	}
	if got := w.Len(); got > total {
		t.Fatalf("容量上限被突破：记录数 %d 超过 %d", got, total)
	}

	// 推进两个窗口使条目可被清理，再写入新 IP，仍不得突破上限
	clock.Advance(3 * time.Minute)
	for i := 0; i < ShardCount*2; i++ {
		w.Hit(fmt.Sprintf("30.0.%d.%d", i/256, i%256))
	}
	if got := w.Len(); got > total {
		t.Fatalf("清理后容量上限被突破：记录数 %d 超过 %d", got, total)
	}
}

func TestShardDistribution(t *testing.T) {
	w := New(Config{Limit: -1, Window: time.Minute})
	defer w.Close()

	const ips = 20000
	counts := make([]int, ShardCount)
	for i := 0; i < ips; i++ {
		ip := fmt.Sprintf("172.16.%d.%d", (i/256)%256, i%256)
		counts[w.shardIndex(ip)]++
	}
	// 均匀哈希下每桶期望 312，允许 ±35% 偏差
	min, max := ips/ShardCount*65/100, ips/ShardCount*135/100
	for i, c := range counts {
		if c < min || c > max {
			t.Fatalf("第 %d 桶分布异常：%d，期望在 [%d, %d]", i, c, min, max)
		}
	}
}

// shardIndex 仅供测试观测分片分布，返回 IP 所在桶的下标。
func (w *Window) shardIndex(ip string) int {
	s := w.shard(ip)
	for i := range w.shards {
		if s == &w.shards[i] {
			return i
		}
	}
	return -1
}

func TestJanitorRemovesExpired(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 1, Window: time.Millisecond, Now: clock.Now})
	defer w.Close()

	const ips = 200
	for i := 0; i < ips; i++ {
		w.Hit(fmt.Sprintf("40.0.0.%d", i))
	}
	clock.Advance(10 * time.Millisecond)

	// 手动触发一轮完整清理（janitor 每秒只清一个桶，测试不等真实时钟）
	w.cleanupAll()
	if got := w.Len(); got != 0 {
		t.Fatalf("过期条目应被清理，剩余 %d", got)
	}
}

func TestJanitorKeepsChallenged(t *testing.T) {
	clock := newFakeClock()
	w := New(Config{Limit: 1, Window: time.Millisecond, Stateful: true, Now: clock.Now})
	defer w.Close()

	w.Hit("50.0.0.1")
	w.Hit("50.0.0.1") // 触发挑战
	clock.Advance(10 * time.Millisecond)
	w.cleanupAll()

	if got := w.Len(); got != 1 {
		t.Fatalf("挑战中的条目不因过期被删除，剩余应为 1，实际 %d", got)
	}
	if got := w.State("50.0.0.1"); got != StateChallenged {
		t.Fatalf("状态应保持 StateChallenged，实际 %v", got)
	}
}

// cleanupAll 是 janitor 的同步版本，仅测试使用。
func (w *Window) cleanupAll() {
	now := w.now().UnixNano()
	for i := range w.shards {
		s := &w.shards[i]
		s.mu.Lock()
		for k, e := range s.m {
			expired := now-e.windowAt.Load() > w.windowNs*2
			normal := !w.stateful || State(e.state.Load()) == StateNormal
			if expired && normal {
				delete(s.m, k)
			}
		}
		s.mu.Unlock()
	}
}

func TestUnlimitedCountsButAllows(t *testing.T) {
	w := New(Config{Limit: 0, Window: time.Minute})
	defer w.Close()

	for i := 1; i <= 5; i++ {
		count, allowed := w.Hit("60.0.0.1")
		if !allowed || count != int64(i) {
			t.Fatalf("Limit=0 应始终放行并计数：count=%d allowed=%v", count, allowed)
		}
	}
}

func BenchmarkHit(b *testing.B) {
	w := New(Config{Limit: 1000000, Window: time.Minute})
	defer w.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Hit("10.0.0.1")
	}
}

func BenchmarkHitParallel(b *testing.B) {
	w := New(Config{Limit: 100000000, Window: time.Minute})
	defer w.Close()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			w.Hit(fmt.Sprintf("10.0.%d.%d", i/256, i%256))
			i++
		}
	})
}
