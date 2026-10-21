package scheduler

import (
	"os"
	"strings"
	"testing"

	"rental-server/logger"

	"github.com/rs/zerolog"
)

func TestMain(m *testing.M) {
	logger.Log = zerolog.Nop()
	os.Exit(m.Run())
}

func newTestRegistry(jobs ...*Job) *Registry {
	r := NewRegistry()
	for _, j := range jobs {
		j.Enabled = true
		j.Time = "03:00"
		r.Add(j)
	}
	return r
}

func TestRunUnknownJob(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Run("nope"); err == nil || !strings.Contains(err.Error(), "未知") {
		t.Fatalf("期望未知任务错误，got %v", err)
	}
}

func TestRunRecordsLastRun(t *testing.T) {
	r := newTestRegistry(&Job{Name: "demo", Title: "示例", Run: func() string { return "完成 1 条" }})

	result, err := r.Run("demo")
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if result != "完成 1 条" {
		t.Fatalf("结果不符: %q", result)
	}
	views := r.List()
	if len(views) != 1 {
		t.Fatalf("任务数不符: %d", len(views))
	}
	v := views[0]
	if v.LastResult != "完成 1 条" || v.Trigger != triggerManual || v.LastRunAt == "" {
		t.Fatalf("最近执行记录不符: %+v", v)
	}
	if v.Schedule != "每天 03:00" {
		t.Fatalf("调度描述不符: %q", v.Schedule)
	}
}

func TestRunAllPreservesOrder(t *testing.T) {
	var order []string
	r := newTestRegistry(
		&Job{Name: "a", Title: "甲", Run: func() string { order = append(order, "a"); return "a" }},
		&Job{Name: "b", Title: "乙", Run: func() string { order = append(order, "b"); return "b" }},
	)
	results := r.RunAll()
	if strings.Join(order, ",") != "a,b" {
		t.Fatalf("执行顺序不符: %v", order)
	}
	if len(results) != 2 || !strings.HasPrefix(results[0], "【甲】") || !strings.HasPrefix(results[1], "【乙】") {
		t.Fatalf("结果格式不符: %v", results)
	}
}

func TestRunRejectsConcurrent(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	r := newTestRegistry(&Job{Name: "slow", Title: "慢任务", Run: func() string {
		close(started)
		<-release
		return "done"
	}})

	done := make(chan struct{})
	go func() {
		_, _ = r.Run("slow")
		close(done)
	}()
	<-started
	if _, err := r.Run("slow"); err == nil || !strings.Contains(err.Error(), "正在执行中") {
		t.Fatalf("期望拒绝并发执行，got %v", err)
	}
	close(release)
	<-done

	// 执行结束后可再次触发
	if _, err := r.Run("slow"); err != nil {
		t.Fatalf("结束后应可再次执行: %v", err)
	}
}

func TestRunRecoversPanic(t *testing.T) {
	r := newTestRegistry(&Job{Name: "boom", Title: "炸", Run: func() string { panic("boom") }})
	result, err := r.Run("boom")
	if err != nil {
		t.Fatalf("panic 应被捕获而不是报错: %v", err)
	}
	if !strings.Contains(result, "panic") {
		t.Fatalf("结果应说明 panic: %q", result)
	}
}

func TestStartupTrigger(t *testing.T) {
	r := newTestRegistry(&Job{Name: "started", Title: "启动任务", Run: func() string { return "ok" }})
	if _, err := r.run("started", triggerStartup); err != nil {
		t.Fatalf("启动执行失败: %v", err)
	}
	if got := r.List()[0].Trigger; got != triggerStartup {
		t.Fatalf("触发来源不符: %q", got)
	}
}
