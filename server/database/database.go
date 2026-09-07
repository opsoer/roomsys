// Package database 统一封装存储后端的选择与初始化。
//
// 目前支持两种后端，由配置项 db_driver 决定，进程启动时二选一：
//   - mysql  （默认，兼容既有部署）：独立 MySQL 服务，走 TCP 连接
//   - sqlite （单机推荐）：进程内嵌入式数据库，单文件零运维
//
// 新增后端（如 PostgreSQL）时的扩展点：
//  1. config 包加 Driver 常量并在 normalize 里放行
//  2. 本文件加一个 openXxx 函数并在 Open 的 switch 里加 case
//  3. 方言差异的 SQL 片段在本包补充对应分支（参考 DayExpr）
package database

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"rental-server/config"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Open 按配置选择存储后端并打开连接，返回的 *gorm.DB 对上层完全透明，
// 业务代码不感知底层是哪种数据库。
func Open(cfg *config.Config) (*gorm.DB, error) {
	switch cfg.DBDriver {
	case config.DriverMySQL:
		return openMySQL(cfg)
	case config.DriverSQLite:
		return openSQLite(cfg.DBPath)
	default:
		return nil, fmt.Errorf("不支持的 db_driver: %q（可选 mysql / sqlite）", cfg.DBDriver)
	}
}

// openMySQL 连接独立 MySQL 服务，连接参数与池大小沿用原有线上配置。
func openMySQL(cfg *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

// openSQLite 打开单文件 SQLite 数据库。
//
// PRAGMA 直接挂 in DSN：连接池里每条新连接建立时都会自动执行，比连接后手动
// Exec 一次更可靠（后者只作用于当时拿到的那条连接）：
//   - journal_mode=WAL    读写不互斥，并发查询不被写事务阻塞
//   - synchronous=NORMAL  WAL 模式推荐级别，大幅减少 fsync 次数
//   - busy_timeout=5000   SQLite 任意时刻仅一个写者，写锁竞争时等待 5s 而非立刻报错
//   - foreign_keys=1      模型里的 OnDelete:CASCADE / SET NULL 依赖它（SQLite 默认关闭）
func openSQLite(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建 SQLite 目录失败: %w", err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)",
		filepath.ToSlash(path))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// DSN 里的 PRAGMA 拼写错误会静默失效，启动时显式校验一次
	var mode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
		return nil, fmt.Errorf("校验 SQLite journal_mode 失败: %w", err)
	}
	if mode != "wal" {
		return nil, fmt.Errorf("SQLite WAL 模式未生效（journal_mode=%s），请检查磁盘/文件系统", mode)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// 进程内连接无网络开销与老化问题，池不宜过大：写路径是单写者，
	// 连接再多也只是排队，反而放大锁竞争
	maxConns := runtime.NumCPU() * 2
	if maxConns < 4 {
		maxConns = 4
	}
	sqlDB.SetMaxOpenConns(maxConns)
	sqlDB.SetMaxIdleConns(maxConns)
	return db, nil
}

// DayExpr 返回把时间列截断到"本地日期"的 SQL 表达式，用于按天 GROUP BY。
//
// MySQL 用 DATE(col)（datetime 列存的就是本地墙上时间）。
// SQLite 的驱动把 time.Time 存为带本地时区偏移的文本（如
// "2026-09-07 00:30:00+08:00"），内置 DATE() 会先换算成 UTC——本地凌晨
// 0~8 点的记录会被归到前一天，因此取文本前 10 位（即本地日期）。
func DayExpr(db *gorm.DB, col string) string {
	if db.Dialector.Name() == "sqlite" {
		return fmt.Sprintf("SUBSTR(%s, 1, 10)", col)
	}
	return fmt.Sprintf("DATE(%s)", col)
}
