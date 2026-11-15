// Package envcheck 在进程启动阶段对关键组件做只读自检并打印结果。
//
// 设计原则：
//   - 只报告不阻断：任何检查失败都不会让进程退出，也不影响后续启动流程；
//   - 每项检查有独立超时与 panic 恢复，单个组件异常不会拖垮整轮自检；
//   - 状态分 OK / WARN / ERROR / SKIP 四级，未配置（如未启用七牛）记 SKIP 而非 ERROR。
package envcheck

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rental-server/config"
	"rental-server/logger"

	"gorm.io/gorm"
)

// 自检状态。
const (
	StatusOK    = "OK"
	StatusWarn  = "WARN"
	StatusError = "ERROR"
	StatusSkip  = "SKIP"
)

// ErrSkip 表示该组件未配置/未启用，检查结果记为 SKIP（不算异常）。
var ErrSkip = errors.New("未启用")

// warning 表示「组件可用但有风险」，检查结果记为 WARN 而不是 ERROR。
type warning struct{ msg string }

func (w warning) Error() string { return w.msg }

// Warnf 让检查主动标记为警告：组件可用，但存在需要关注的风险。
func Warnf(format string, args ...any) error {
	return warning{msg: fmt.Sprintf(format, args...)}
}

// Check 单个组件检查：返回人类可读明细；err 非空表示不可用或存在风险。
type Check struct {
	Name string
	Run  func(ctx context.Context) (detail string, err error)
}

// Result 单个组件的检查结果。
type Result struct {
	Name   string
	Status string
	Detail string
}

// Run 依次执行全部检查并打印结果，返回结果列表（供测试与后续扩展）。
// cfg.Server.StartupCheck 显式 enabled=false 时整轮跳过。
func Run(cfg *config.Config, db *gorm.DB, version string) []Result {
	if !cfg.Server.StartupCheck.IsEnabled() {
		logger.Log.Info().Msg("启动环境自检已按配置关闭（server.startup_check.enabled=false）")
		return nil
	}

	timeout := time.Duration(cfg.Server.StartupCheck.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	checks := checksFor(cfg, db, version)
	logger.Log.Info().Msg("========== 启动环境自检 ==========")
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		res := runOne(c, timeout)
		printResult(res)
		results = append(results, res)
	}
	printSummary(results)
	return results
}

// runOne 执行单个检查：panic 与超时都归一化为 ERROR，绝不影响调用方。
func runOne(c Check, timeout time.Duration) (res Result) {
	res = Result{Name: c.Name, Status: StatusError, Detail: "检查未返回结果"}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type outcome struct {
		detail string
		err    error
	}
	ch := make(chan outcome, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				ch <- outcome{err: fmt.Errorf("panic: %v", rec)}
			}
		}()
		detail, err := c.Run(ctx)
		ch <- outcome{detail: detail, err: err}
	}()

	select {
	case o := <-ch:
		res.Detail = o.detail
		switch {
		case errors.Is(o.err, ErrSkip):
			res.Status = StatusSkip
			if res.Detail == "" {
				res.Detail = "未配置"
			}
		case o.err != nil:
			var w warning
			if errors.As(o.err, &w) {
				res.Status = StatusWarn
				res.Detail = joinDetail(o.detail, w.msg)
			} else {
				res.Status = StatusError
				res.Detail = joinDetail(o.detail, o.err.Error())
			}
		default:
			res.Status = StatusOK
			if res.Detail == "" {
				res.Detail = "正常"
			}
		}
	case <-ctx.Done():
		res.Status = StatusError
		res.Detail = fmt.Sprintf("检查超时（>%s）", timeout)
	}
	return res
}

// joinDetail 拼接检查明细与错误/警告信息。
func joinDetail(detail, msg string) string {
	if msg == "" {
		return detail
	}
	if detail == "" {
		return msg
	}
	return detail + "；" + msg
}

// printResult 按状态选择日志级别输出，控制台与日志文件同步可见。
func printResult(r Result) {
	msg := fmt.Sprintf("[%s] %s：%s", r.Status, r.Name, r.Detail)
	switch r.Status {
	case StatusError:
		logger.Log.Error().Str("component", r.Name).Str("status", r.Status).Msg(msg)
	case StatusWarn:
		logger.Log.Warn().Str("component", r.Name).Str("status", r.Status).Msg(msg)
	default:
		logger.Log.Info().Str("component", r.Name).Str("status", r.Status).Msg(msg)
	}
}

// summary 汇总各状态数量。
type summary struct {
	ok, warn, err, skip int
}

// summarize 统计结果状态数量。
func summarize(results []Result) summary {
	var s summary
	for _, r := range results {
		switch r.Status {
		case StatusOK:
			s.ok++
		case StatusWarn:
			s.warn++
		case StatusError:
			s.err++
		case StatusSkip:
			s.skip++
		}
	}
	return s
}

// printSummary 输出自检汇总：明确说明不阻断启动。
func printSummary(results []Result) {
	s := summarize(results)
	logger.Log.Info().
		Int("ok", s.ok).Int("warn", s.warn).Int("error", s.err).Int("skip", s.skip).
		Msgf("自检完成：%d 正常 / %d 警告 / %d 错误 / %d 跳过（不阻断启动）", s.ok, s.warn, s.err, s.skip)
}
