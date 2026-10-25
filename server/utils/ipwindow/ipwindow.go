// Package ipwindow 提供按 IP 分片的滑动窗口计数器。
//
// 背景：项目里有多处「按 IP 计数」的需求（API 限流、查看房东电话的免费额度、
// 登录失败次数），早期实现是「一把全局 sync.Mutex + 一个 map」，在高并发下
// 所有请求争同一把锁，且定时清理要全表遍历并全程持锁。本包把它们统一成：
//
//   - 64 个分片桶，按 IP 哈希取模落桶，锁竞争降低到 1/64；
//   - 桶内 map 结构由桶锁保护，计数与窗口时间用原子字段，读路径更轻；
//   - 窗口切换在桶锁内完成，避免「两个请求同时判定过期、各自重置」导致多放行；
//   - 每桶独立清理（janitor 轮转），不会长时间阻塞全部请求；
//   - 每桶容量上限，超出后淘汰最旧条目，防止伪造 IP 把内存撑爆。
//
// 用法（纯限流）：
//
//	w := ipwindow.New(ipwindow.Config{Limit: 240, Window: time.Minute})
//	if _, allowed := w.Hit(ip); !allowed { /* 429 */ }
//
// 用法（带状态机的额度控制，如手机号查看的验证码）：
//
//	w := ipwindow.New(ipwindow.Config{Limit: 5, Window: time.Minute, Stateful: true})
//	if _, allowed := w.Hit(ip); !allowed || w.State(ip) == ipwindow.StateChallenged {
//	    if !verifyCaptcha() {
//	        w.Fail(ip)
//	        return
//	    }
//	    w.Resolve(ip)
//	}
package ipwindow

import (
	"sync"
	"sync/atomic"
	"time"
)

// State 是额度状态机的状态；仅当 Config.Stateful 为 true 时使用。
type State int32

const (
	// StateNormal 正常：窗口内计数未超限，直接放行。
	StateNormal State = iota
	// StateChallenged 已触发阈值：之后的每次请求都必须通过验证码，
	// 直到 Resolve 成功才回到 StateNormal（粘性，不随时间自动解除）。
	StateChallenged
)

// Config 是窗口计数器的配置。
type Config struct {
	// Limit 一个窗口内允许的请求次数；0 表示不限流（仍计数）。
	Limit int64
	// Window 滑动窗口长度，必须大于 0。
	Window time.Duration
	// MaxSizePerShard 每个桶最多保存的 IP 条目数，<=0 时使用默认值 4096。
	// 64 桶 × 4096 ≈ 26 万个 IP 的计数上限。
	MaxSizePerShard int
	// Stateful 为 true 时启用「触发阈值后粘性挑战」状态机（验证码场景）。
	Stateful bool
	// ResolveKeepRatio 验证码校验成功后保留的计数比例（百分比，0~100）。
	// 100 表示不清零，50 表示减半。仅在 Stateful 时生效；默认 50。
	// 不清零的原因：若验证成功即归零，爬虫可用 OCR 换「再来 Limit 次免费额度」，
	// 成本固定且很低；减半后查看量越大，验证码次数越多，OCR 成本线性增长。
	ResolveKeepRatio int
	// Now 可注入的时钟，便于测试；为 nil 时使用 time.Now。
	Now func() time.Time
}

const (
	// ShardCount 分片数量，固定 64（取模用位运算，扩容需要迁移数据）。
	ShardCount = 64
	// defaultMaxSizePerShard 每桶默认容量上限。
	defaultMaxSizePerShard = 4096
	// janitorInterval 清理协程的轮转间隔：每秒清一个桶，64 秒扫完一圈。
	janitorInterval = time.Second
)

// entry 是单个 IP 的计数记录。指针一旦创建就不再替换（删除只是从 map 移除），
// 因此正在使用旧指针的请求不会拿到 nil，也不会读到「复活」的旧计数。
type entry struct {
	count     atomic.Int64
	windowAt  atomic.Int64 // 窗口起始时间，UnixNano
	state     atomic.Int32
	lastTouch atomic.Int64 // 最近一次访问，UnixNano，用于容量淘汰选出最旧条目
}

type shard struct {
	mu sync.Mutex
	m  map[string]*entry
}

// Window 是按 IP 分片的窗口计数器。
type Window struct {
	shards   [ShardCount]shard
	limit    int64
	windowNs int64
	maxSize  int
	stateful bool
	keepPct  int
	now      func() time.Time
	stopOnce sync.Once
	stop     chan struct{}
}

// New 创建并启动后台清理协程。
func New(cfg Config) *Window {
	if cfg.Window <= 0 {
		panic("ipwindow: Window 必须大于 0")
	}
	w := &Window{
		limit:    cfg.Limit,
		windowNs: int64(cfg.Window),
		maxSize:  cfg.MaxSizePerShard,
		stateful: cfg.Stateful,
		keepPct:  cfg.ResolveKeepRatio,
		now:      cfg.Now,
		stop:     make(chan struct{}),
	}
	if w.maxSize <= 0 {
		w.maxSize = defaultMaxSizePerShard
	}
	if w.now == nil {
		w.now = time.Now
	}
	if w.keepPct <= 0 || w.keepPct > 100 {
		w.keepPct = 50
	}
	for i := range w.shards {
		w.shards[i].m = make(map[string]*entry)
	}
	go w.janitor()
	return w
}

// Close 停止后台清理协程；主要供测试使用。
func (w *Window) Close() {
	w.stopOnce.Do(func() { close(w.stop) })
}

// Hit 记录一次访问并返回本窗口内的计数与是否放行。
//
// Stateful 模式下：
//   - 计数从 0 递增到 Limit 的这段请求返回 allowed=true（免费额度）；
//   - 计数超过 Limit 时把状态切成 StateChallenged 并返回 allowed=false；
//   - 一旦进入 StateChallenged，即使计数因减半回落，也持续返回 false。
func (w *Window) Hit(ip string) (count int64, allowed bool) {
	s := w.shard(ip)
	now := w.now().UnixNano()

	s.mu.Lock()
	e := s.m[ip]
	if e == nil {
		// map 达到上限时先清理过期项，仍然满则淘汰最旧条目
		if len(s.m) >= w.maxSize {
			w.evictLocked(s, now)
		}
		e = &entry{}
		e.windowAt.Store(now)
		s.m[ip] = e
	} else if now-e.windowAt.Load() >= w.windowNs {
		// 窗口切换与计数重置必须在同一把桶锁内，否则并发请求会重复放行
		e.windowAt.Store(now)
		e.count.Store(0)
	}
	e.lastTouch.Store(now)
	c := e.count.Add(1)
	challenged := w.stateful && e.state.Load() == int32(StateChallenged)
	if w.limit > 0 && c > w.limit {
		if w.stateful {
			e.state.Store(int32(StateChallenged))
			challenged = true
		}
	}
	s.mu.Unlock()

	return c, !challenged && (w.limit <= 0 || c <= w.limit)
}

// State 返回 IP 当前状态（仅 Stateful 模式有意义）。
func (w *Window) State(ip string) State {
	s := w.shard(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[ip]; e != nil {
		return State(e.state.Load())
	}
	return StateNormal
}

// Count 返回 IP 在当前窗口内已记录的次数，不产生计数。
func (w *Window) Count(ip string) int64 {
	s := w.shard(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[ip]
	if e == nil {
		return 0
	}
	if w.now().UnixNano()-e.windowAt.Load() >= w.windowNs {
		return 0
	}
	return e.count.Load()
}

// Resolve 处理「验证码校验成功」：状态回到 StateNormal，计数按 keepPct 保留。
// 返回保留后的计数。仅 Stateful 模式生效。
func (w *Window) Resolve(ip string) int64 {
	s := w.shard(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[ip]
	if e == nil {
		return 0
	}
	kept := e.count.Load() * int64(w.keepPct) / 100
	e.count.Store(kept)
	e.state.Store(int32(StateNormal))
	e.lastTouch.Store(w.now().UnixNano())
	return kept
}

// Challenged 供失败路径使用：确保 IP 处于 StateChallenged。
// 仅 Stateful 模式生效。
func (w *Window) Challenged(ip string) {
	if !w.stateful {
		return
	}
	s := w.shard(ip)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[ip]
	if e == nil {
		e = &entry{}
		e.windowAt.Store(w.now().UnixNano())
		s.m[ip] = e
	}
	e.state.Store(int32(StateChallenged))
}

// Reset 删除 IP 的全部记录（运营手动解封、测试使用）。
func (w *Window) Reset(ip string) {
	s := w.shard(ip)
	s.mu.Lock()
	delete(s.m, ip)
	s.mu.Unlock()
}

// Len 返回当前记录的 IP 总数（调试与测试使用）。
func (w *Window) Len() int {
	total := 0
	for i := range w.shards {
		s := &w.shards[i]
		s.mu.Lock()
		total += len(s.m)
		s.mu.Unlock()
	}
	return total
}

// shard 按 FNV-1a 32 位哈希选择桶，避免大量同网段 IP 落到同一个桶。
func (w *Window) shard(ip string) *shard {
	const (
		offset32 = 2166136261
		prime32  = 16777619
	)
	h := uint32(offset32)
	for i := 0; i < len(ip); i++ {
		h ^= uint32(ip[i])
		h *= prime32
	}
	return &w.shards[h&(ShardCount-1)]
}

// evictLocked 在桶已满时腾出空间：优先删过期且不在挑战状态的条目，
// 仍然满则删最近最少访问的一条。调用方必须持有 s.mu。
func (w *Window) evictLocked(s *shard, now int64) {
	var oldestKey string
	var oldestTouch int64 = -1
	for k, e := range s.m {
		expired := now-e.windowAt.Load() >= w.windowNs*2
		normal := !w.stateful || State(e.state.Load()) == StateNormal
		if expired && normal {
			delete(s.m, k)
			continue
		}
		touch := e.lastTouch.Load()
		if oldestTouch < 0 || touch < oldestTouch {
			oldestTouch = touch
			oldestKey = k
		}
	}
	// 清理过期项后若仍达到上限，再淘汰最久未访问的一条，保证插入后不超过上限
	if len(s.m) >= w.maxSize && oldestKey != "" {
		delete(s.m, oldestKey)
	}
}

// janitor 每秒清一个桶，64 秒覆盖全部桶；单次只锁一个桶，不会阻塞其他请求。
func (w *Window) janitor() {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	idx := 0
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			s := &w.shards[idx&(ShardCount-1)]
			idx++
			now := w.now().UnixNano()
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
}
