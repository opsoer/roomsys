// resetdb 清空本地数据库：删除当前库中的全部表（包括存放 root 账号的 users 表）。
// 清完后无需手工做任何事——下次启动后端进程时，启动流程里的 AutoMigrate 会把全部表
// 重建出来，并自动创建超级管理员 root/root，即得到一个与全新部署等效的干净环境。
//
// 支持双存储后端，按 config.json 的 db_driver 连接对应数据库：
//   - mysql  ：清空 MySQL 库中查到的全部表
//   - sqlite ：清空 SQLite 文件中的全部表（保留文件本身，表结构与数据一并清空）
//
// 使用场景：
//   - 多轮测试之间清库，保证每一轮从干净状态开始，上一轮的残留数据不影响下一轮
//   - 重跑 scripts/seed 前清库（seed 要求公客单恰好为空）
//   - 演示数据灌坏了、想推倒重来
//
// 流程与原理：
//  1. config.Load() 读取 config.json 拿数据库连接参数（因此必须在 server 目录下运行）
//  2. 动态查出当前库的全部表——MySQL 查 information_schema.tables，SQLite 查
//     sqlite_master——不硬编码模型清单，以后 models 增删表也不会漏清
//     （旧版 drop_all 脚本就漏了 BuildingRenewal/AuditLog/PageView 三张表）
//  3. 绕开外键依赖顺序逐表 DROP：MySQL 用 SET FOREIGN_KEY_CHECKS=0；
//     SQLite 在单条专用连接上 PRAGMA foreign_keys=OFF（PRAGMA 是连接级开关，
//     连接池下必须固定在同一连接执行才可靠）
//  4. 只删表、不动数据库本身；users 表一并删除，root 由下次启动自动重建
//
// 安全机制：
//   - 默认交互确认：需要手动输入 yes 才执行；加 -y 或 --yes 参数跳过（CI/脚本场景）
//   - 执行前打印目标实例（MySQL host/库名或 SQLite 文件路径）与即将删除的表数量
//
// 参数：
//
//	-y, --yes  跳过交互确认
//	--media    额外清空本地媒体上传目录（config.json 的 upload_dir，默认 ./storage/media）。
//	           注意七牛上的对象不受影响（配置了七牛时文件实际存在云端，本地目录可能为空），
//	           清云端请到七牛控制台操作
//
// 运行方式（在 server 目录下）：
//
//	go run ./scripts/resetdb              # 交互确认后清库
//	go run ./scripts/resetdb -y           # 跳过确认
//	go run ./scripts/resetdb -y --media   # 连本地媒体文件一起清
//
// 建议流程：先停掉后端进程 → 清库 → 启动后端（自动重建表 + root/root）→ 需要演示数据再跑 seed。
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"rental-server/config"
	"rental-server/database"

	"gorm.io/gorm"
)

func main() {
	yes := false
	cleanMedia := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "-y", "--yes":
			yes = true
		case "--media":
			cleanMedia = true
		}
	}

	cfg := config.Load()
	db, err := database.Open(cfg)
	if err != nil {
		panic(fmt.Sprintf("连接数据库失败: %v", err))
	}

	// 动态查出当前库全部表，不依赖 models 清单，避免以后增删表时漏清
	tables, err := listTables(db, cfg)
	if err != nil {
		panic(fmt.Sprintf("查询表清单失败: %v", err))
	}
	if len(tables) == 0 {
		fmt.Println("数据库已是空的，无需清理")
		return
	}

	if cfg.DBDriver == config.DriverSQLite {
		fmt.Printf("目标：SQLite 文件 %s\n", cfg.DBPath)
	} else {
		fmt.Printf("目标：%s:%s / 库 %s\n", cfg.DBHost, cfg.DBPort, cfg.DBName)
	}
	fmt.Printf("即将删除 %d 张表（含 root 账号），此操作不可恢复：\n", len(tables))
	for i, t := range tables {
		fmt.Printf("  %s", t)
		if (i+1)%5 == 0 || i == len(tables)-1 {
			fmt.Println()
		}
	}

	if !yes {
		fmt.Print("\n确认请输入 yes：")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) != "yes" {
			fmt.Println("已取消")
			return
		}
	}

	dropped := 0
	if cfg.DBDriver == config.DriverSQLite {
		dropped, err = dropTablesSQLite(db, tables)
	} else {
		dropped, err = dropTablesMySQL(db, tables)
	}
	if err != nil {
		panic(err)
	}
	fmt.Printf("已删除 %d/%d 张表\n", dropped, len(tables))

	if cleanMedia {
		if err := os.RemoveAll(cfg.UploadDir); err != nil {
			fmt.Printf("清空媒体目录 %s 失败: %v\n", cfg.UploadDir, err)
		} else {
			fmt.Printf("已清空本地媒体目录 %s（七牛云端文件不受影响）\n", cfg.UploadDir)
		}
	}

	fmt.Println("\n完成。启动后端进程即可：表结构会由 AutoMigrate 自动重建，root/root 自动创建。")
}

// listTables 按方言返回当前库中全部表名。
func listTables(db *gorm.DB, cfg *config.Config) ([]string, error) {
	if cfg.DBDriver == config.DriverSQLite {
		return listTablesSQLite(db)
	}
	return listTablesMySQL(db, cfg.DBName)
}

// dropTablesMySQL 关闭外键检查后逐表 DROP，规避表间依赖顺序。
func dropTablesMySQL(db *gorm.DB, tables []string) (int, error) {
	if err := db.Exec("SET FOREIGN_KEY_CHECKS=0").Error; err != nil {
		return 0, fmt.Errorf("关闭外键检查失败: %w", err)
	}
	dropped := 0
	for _, t := range tables {
		if err := db.Exec("DROP TABLE IF EXISTS `" + t + "`").Error; err != nil {
			fmt.Printf("删除表 %s 失败: %v\n", t, err)
			continue
		}
		dropped++
	}
	db.Exec("SET FOREIGN_KEY_CHECKS=1")
	return dropped, nil
}

// dropTablesSQLite 在单条专用连接上关外键后逐表 DROP：PRAGMA foreign_keys 是
// 连接级开关，走连接池的 db.Exec 无法保证落在同一条连接上；全部清完后把 WAL
// 日志并回主文件，留下一个干净的单文件库。
func dropTablesSQLite(db *gorm.DB, tables []string) (int, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return 0, err
	}
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
		return 0, fmt.Errorf("关闭外键检查失败: %w", err)
	}
	dropped := 0
	for _, t := range tables {
		if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS "`+t+`"`); err != nil {
			fmt.Printf("删除表 %s 失败: %v\n", t, err)
			continue
		}
		dropped++
	}
	conn.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	conn.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	return dropped, nil
}

// listTablesMySQL 从 information_schema 查全部基表（排除视图）。
func listTablesMySQL(db *gorm.DB, schema string) ([]string, error) {
	rows, err := db.Raw(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = ? AND table_type = 'BASE TABLE'",
		schema,
	).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// listTablesSQLite 从 sqlite_master 查全部用户表（排除 sqlite_ 内部表与视图）。
func listTablesSQLite(db *gorm.DB) ([]string, error) {
	rows, err := db.Raw(
		"SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'",
	).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}
