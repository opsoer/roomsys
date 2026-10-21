package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rental-server/logger"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// slowQueryThreshold 慢查询阈值：超过该耗时的 SQL 以 Warn 级别记录（含 SQL 文本）。
const slowQueryThreshold = 200 * time.Millisecond

// gormLogger 把 GORM 日志接入 zerolog，统一全局日志级别与格式：
//   - 常规 SQL 不输出（查询日志噪音大，需要排查时把 server.log_level 调成 debug）；
//   - "record not found" 属正常业务分支（查不到就返回 404），不记日志；
//   - 唯一键冲突等可预期错误记 Warn，其余错误记 Error；
//   - 慢查询（>200ms）记 Warn 并附 SQL 文本，便于定位性能问题。
type gormLogger struct {
	level gormlogger.LogLevel
}

// LogMode 实现 gormlogger.Interface。
func (l gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	l.level = level
	return l
}

// Info 实现 gormlogger.Interface：GORM 的信息类输出统一降为 Debug。
func (l gormLogger) Info(_ context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Info {
		logger.Log.Debug().Str("gorm", fmt.Sprintf(msg, args...)).Msg("GORM")
	}
}

// Warn 实现 gormlogger.Interface。
func (l gormLogger) Warn(_ context.Context, msg string, args ...interface{}) {
	logger.Log.Warn().Str("gorm", fmt.Sprintf(msg, args...)).Msg("GORM")
}

// Error 实现 gormlogger.Interface。
func (l gormLogger) Error(_ context.Context, msg string, args ...interface{}) {
	logger.Log.Error().Str("gorm", fmt.Sprintf(msg, args...)).Msg("GORM")
}

// Trace 实现 gormlogger.Interface：按 SQL 执行结果分级记录。
func (l gormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)
	switch {
	case err == nil:
		if elapsed > slowQueryThreshold && l.level >= gormlogger.Warn {
			sql, rows := fc()
			logger.Log.Warn().
				Dur("elapsed", elapsed).
				Int64("rows", rows).
				Str("sql", sql).
				Msg("慢查询")
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		// 查不到是正常业务分支（多数接口据此返回 404），不记日志
	case errors.Is(err, gorm.ErrDuplicatedKey):
		sql, _ := fc()
		logger.Log.Warn().Dur("elapsed", elapsed).Str("sql", sql).Msg("数据库唯一约束冲突")
	default:
		sql, _ := fc()
		logger.Log.Error().Err(err).Dur("elapsed", elapsed).Str("sql", sql).Msg("数据库执行失败")
	}
}

// newGormConfig 返回统一的 GORM 配置（静默常规 SQL，错误走 zerolog）。
func newGormConfig() *gorm.Config {
	return &gorm.Config{Logger: gormLogger{level: gormlogger.Warn}}
}
