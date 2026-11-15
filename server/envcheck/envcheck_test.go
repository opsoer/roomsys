package envcheck

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rental-server/config"
)

func TestRunOneOK(t *testing.T) {
	res := runOne(Check{Name: "测试", Run: func(context.Context) (string, error) { return "一切正常", nil }}, time.Second)
	if res.Status != StatusOK || res.Detail != "一切正常" {
		t.Fatalf("期望 OK/一切正常，得到 %+v", res)
	}
}

func TestRunOneError(t *testing.T) {
	res := runOne(Check{Name: "测试", Run: func(context.Context) (string, error) { return "明细", errors.New("boom") }}, time.Second)
	if res.Status != StatusError || !strings.Contains(res.Detail, "明细") || !strings.Contains(res.Detail, "boom") {
		t.Fatalf("期望 ERROR 且同时包含明细与错误，得到 %+v", res)
	}
}

func TestRunOneWarn(t *testing.T) {
	res := runOne(Check{Name: "测试", Run: func(context.Context) (string, error) { return "可用", Warnf("有风险") }}, time.Second)
	if res.Status != StatusWarn || res.Detail != "可用；有风险" {
		t.Fatalf("期望 WARN/可用；有风险，得到 %+v", res)
	}
}

func TestRunOneSkip(t *testing.T) {
	res := runOne(Check{Name: "测试", Run: func(context.Context) (string, error) { return "未配置", ErrSkip }}, time.Second)
	if res.Status != StatusSkip || res.Detail != "未配置" {
		t.Fatalf("期望 SKIP/未配置，得到 %+v", res)
	}
}

func TestRunOnePanic(t *testing.T) {
	res := runOne(Check{Name: "测试", Run: func(context.Context) (string, error) { panic("爆炸") }}, time.Second)
	if res.Status != StatusError || !strings.Contains(res.Detail, "爆炸") {
		t.Fatalf("期望 ERROR 且包含 panic 信息，得到 %+v", res)
	}
}

func TestRunOneTimeout(t *testing.T) {
	res := runOne(Check{Name: "测试", Run: func(ctx context.Context) (string, error) {
		select {
		case <-time.After(200 * time.Millisecond):
			return "不该到达", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}, 20*time.Millisecond)
	if res.Status != StatusError || !strings.Contains(res.Detail, "超时") {
		t.Fatalf("期望超时 ERROR，得到 %+v", res)
	}
}

func TestSummarize(t *testing.T) {
	results := []Result{
		{Status: StatusOK}, {Status: StatusOK},
		{Status: StatusWarn}, {Status: StatusError}, {Status: StatusSkip},
	}
	s := summarize(results)
	if s.ok != 2 || s.warn != 1 || s.err != 1 || s.skip != 1 {
		t.Fatalf("汇总数量不符: %+v", s)
	}
}

func TestChecksForNamesUnique(t *testing.T) {
	got := checksFor(&config.Config{}, nil, "dev")
	if len(got) == 0 {
		t.Fatal("检查项不应为空")
	}
	seen := map[string]bool{}
	for _, c := range got {
		if seen[c.Name] {
			t.Fatalf("检查项重名: %s", c.Name)
		}
		seen[c.Name] = true
	}
}

func TestStartupCheckConfigDefaultEnabled(t *testing.T) {
	var c config.StartupCheckConfig
	if !c.IsEnabled() {
		t.Fatal("未配置时应默认启用")
	}
	disabled := false
	c.Enabled = &disabled
	if c.IsEnabled() {
		t.Fatal("显式 false 应关闭")
	}
}

func TestCheckUploadDirWritable(t *testing.T) {
	detail, err := checkUploadDir(context.Background(), t.TempDir())
	if err != nil || detail == "" {
		t.Fatalf("期望可写，得到 detail=%q err=%v", detail, err)
	}
}

func TestCheckWebDist(t *testing.T) {
	dir := t.TempDir()
	if _, err := checkWebDist(dir); err == nil {
		t.Fatal("缺少 index.html 应报错")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := checkWebDist(dir); err == nil {
		t.Fatal("缺少 assets 目录应给出警告")
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if detail, err := checkWebDist(dir); err != nil || detail == "" {
		t.Fatalf("完整产物应通过，得到 detail=%q err=%v", detail, err)
	}
}

func TestCheckPort(t *testing.T) {
	detail, err := checkPort("0")
	if err != nil || !strings.Contains(detail, "可监听") {
		t.Fatalf("端口 0 应可监听，得到 detail=%q err=%v", detail, err)
	}
}
