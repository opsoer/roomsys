// Package scheduler 提供后台定时任务的注册、调度与手动执行能力。
// 任务定义与调度周期来自 config.yaml 的 scheduler 段，运行时不可热更新（改配置需重启）。
package scheduler

import (
	"fmt"
	"sync"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/utils"
)

// Job 单个定时任务定义：元信息 + 执行函数。Run 返回一句执行结果描述。
type Job struct {
	// Name 任务标识（配置键、接口路径参数），如 rent_bills。
	Name string
	// Title 中文名，供管理界面展示。
	Title string
	// Enabled 是否启用自动调度。
	Enabled bool
	// Time 每天定时时刻 "HH:MM"（与 IntervalHours 二选一）。
	Time string
	// IntervalHours 每隔 N 小时执行（Time 为空时生效）。
	IntervalHours int
	// RunOnStart 进程启动时是否先执行一次（保持业务连续性的既有行为）。
	RunOnStart bool
	// Run 实际执行逻辑，返回结果描述文案。
	Run func() string
}

// NewJob 从配置构造任务定义，保证 Time/IntervalHours 等字段映射一致。
func NewJob(name, title string, c config.JobConfig, runOnStart bool, run func() string) *Job {
	return &Job{
		Name:          name,
		Title:         title,
		Enabled:       c.Enabled,
		Time:          c.Time,
		IntervalHours: c.IntervalHours,
		RunOnStart:    runOnStart,
		Run:           run,
	}
}

// LastRunInfo 单个任务最近一次执行记录（内存态，进程重启清空）。
type LastRunInfo struct {
	// At 执行时刻。
	At time.Time
	// Result 执行结果描述。
	Result string
	// DurationMS 耗时（毫秒）。
	DurationMS int64
	// Trigger 触发来源：scheduler / manual。
	Trigger string
}

// Registry 定时任务注册表：持有全部任务定义与最近执行记录。
type Registry struct {
	mu      sync.Mutex
	jobs    []*Job
	lastRun map[string]LastRunInfo
	running map[string]bool
}

// NewRegistry 创建空注册表。
func NewRegistry() *Registry {
	return &Registry{
		lastRun: map[string]LastRunInfo{},
		running: map[string]bool{},
	}
}

// Add 注册任务（进程启动时调用，顺序即展示顺序）。
func (r *Registry) Add(j *Job) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, j)
}

// JobView 面向下端的任务视图（含调度描述与最近执行记录）。
type JobView struct {
	Name       string `json:"name"`
	Title      string `json:"title"`
	Enabled    bool   `json:"enabled"`
	Schedule   string `json:"schedule"`
	LastRunAt  string `json:"last_run_at"`
	LastResult string `json:"last_result"`
	LastMS     int64  `json:"last_ms"`
	Trigger    string `json:"trigger"`
}

// ScheduleDesc 把调度配置渲染为人类可读描述（time 优先，与配置规范一致）。
func (j *Job) ScheduleDesc() string {
	if j.Time != "" {
		return "每天 " + j.Time
	}
	if j.IntervalHours > 0 {
		return fmt.Sprintf("每%d小时", j.IntervalHours)
	}
	return "未配置"
}

// List 返回全部任务视图。
func (r *Registry) List() []JobView {
	r.mu.Lock()
	defer r.mu.Unlock()
	views := make([]JobView, 0, len(r.jobs))
	for _, j := range r.jobs {
		v := JobView{Name: j.Name, Title: j.Title, Enabled: j.Enabled, Schedule: j.ScheduleDesc()}
		if lr, ok := r.lastRun[j.Name]; ok {
			v.LastRunAt = lr.At.Format("2006-01-02 15:04:05")
			v.LastResult = lr.Result
			v.LastMS = lr.DurationMS
			v.Trigger = lr.Trigger
		}
		views = append(views, v)
	}
	return views
}

// 最近一次执行的触发来源取值。
const (
	triggerManual    = "manual"
	triggerScheduler = "scheduler"
	triggerStartup   = "startup"
)

// Run 按名称手动执行单个任务（不受 enabled 限制），返回结果描述。
func (r *Registry) Run(name string) (string, error) {
	return r.run(name, triggerManual)
}

// run 按名称执行任务并记录触发来源。
func (r *Registry) run(name, trigger string) (string, error) {
	r.mu.Lock()
	var job *Job
	for _, j := range r.jobs {
		if j.Name == name {
			job = j
			break
		}
	}
	if job == nil {
		r.mu.Unlock()
		return "", fmt.Errorf("未知的定时任务: %s", name)
	}
	if r.running[name] {
		r.mu.Unlock()
		return "", fmt.Errorf("任务正在执行中: %s", name)
	}
	r.running[name] = true
	r.mu.Unlock()

	result, ms := execJob(job)
	r.finish(name, trigger, result, ms)
	return result, nil
}

// begin 把任务标记为执行中；已在执行则返回 false（供调度循环避免与手动执行并发）。
func (r *Registry) begin(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running[name] {
		return false
	}
	r.running[name] = true
	return true
}

// finish 清除运行标记并记录最近一次执行信息。
func (r *Registry) finish(name, trigger, result string, ms int64) {
	r.mu.Lock()
	r.running[name] = false
	r.lastRun[name] = LastRunInfo{At: utils.Now(), Result: result, DurationMS: ms, Trigger: trigger}
	r.mu.Unlock()
}

// RunAll 顺序执行全部任务（旧「执行全部定时任务」按钮语义），返回逐个结果。
func (r *Registry) RunAll() []string {
	r.mu.Lock()
	jobs := make([]*Job, len(r.jobs))
	copy(jobs, r.jobs)
	r.mu.Unlock()

	results := make([]string, 0, len(jobs))
	for _, j := range jobs {
		res, err := r.Run(j.Name)
		if err != nil {
			res = "跳过: " + err.Error()
		}
		results = append(results, fmt.Sprintf("【%s】%s", j.Title, res))
	}
	return results
}

// execJob 执行任务并计时，panic 捕获为失败结果（与旧调度器行为一致）。
func execJob(j *Job) (result string, ms int64) {
	start := time.Now()
	defer func() {
		if rec := recover(); rec != nil {
			result = fmt.Sprintf("任务 panic: %v", rec)
			logger.Log.Error().Interface("panic", rec).Str("job", j.Name).Msg("定时任务 panic，已恢复")
		}
		ms = time.Since(start).Milliseconds()
		logger.Log.Info().Str("job", j.Name).Str("title", j.Title).Int64("ms", ms).Msg("定时任务执行完成")
	}()
	result = j.Run()
	return
}

// Start 为每个启用的任务启动独立调度协程（daily 按 HH:MM，interval 按每隔 N 小时）。
// RunOnStart 的任务会先经注册表执行一次（保持账单/到期检查尽快生效的原行为）；
// 已停用的任务不参与自动调度，也不在启动时执行（仍可手动触发）。
func (r *Registry) Start() {
	r.mu.Lock()
	jobs := make([]*Job, len(r.jobs))
	copy(jobs, r.jobs)
	r.mu.Unlock()
	for _, j := range jobs {
		if !j.Enabled {
			logger.Log.Info().Str("job", j.Name).Msg("定时任务已停用（enabled=false）")
			continue
		}
		if j.RunOnStart {
			if _, err := r.run(j.Name, triggerStartup); err != nil {
				logger.Log.Warn().Err(err).Str("job", j.Name).Msg("启动时任务执行失败")
			}
		}
		go r.loop(j)
	}
}

// loop 单个任务的调度循环。
func (r *Registry) loop(j *Job) {
	defer func() {
		if rec := recover(); rec != nil {
			logger.Log.Error().Interface("panic", rec).Str("job", j.Name).Msg("定时任务调度协程 panic，已恢复")
		}
	}()
	for {
		var wait time.Duration
		if j.Time != "" {
			wait = untilNextDaily(j.Time)
		} else if j.IntervalHours > 0 {
			wait = time.Duration(j.IntervalHours) * time.Hour
		} else {
			wait = time.Hour
		}
		time.Sleep(wait)

		// 与手动执行互斥：上一轮（或手动触发）尚未结束时跳过本次调度
		if !r.begin(j.Name) {
			logger.Log.Warn().Str("job", j.Name).Msg("任务仍在执行，跳过本次调度")
			continue
		}
		result, ms := execJob(j)
		r.finish(j.Name, triggerScheduler, result, ms)
	}
}

// untilNextDaily 计算距下一个 "HH:MM" 的真实时长（基于模拟时间 utils.Now 保证与业务口径一致）。
func untilNextDaily(timeStr string) time.Duration {
	h, m := 3, 0
	if t, err := time.Parse("15:04", timeStr); err == nil {
		h, m = t.Hour(), t.Minute()
	}
	now := utils.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next.Sub(now)
}
