// export_sqlite 一次性数据迁移工具：把 config.json 所配置 MySQL 库的全部业务表
// 搬运到 SQLite 数据库文件，用于把部署从 MySQL 切换到 SQLite（或迁移到新服务器）。
//
// 用法（在 server 目录下）：
//
//	go run ./cmd/export_sqlite -out ./storage/export.db [-force]
//
// 说明：
//   - 源库永远按 db_host/db_port/db_user/db_password/db_name 走 MySQL
//     （无论 config.json 里 db_driver 当前设为什么）
//   - 目标文件已存在时拒绝执行，-force 可覆盖
//   - 软删除记录一并迁移，主键 ID 原样保留，切换后数据无感
//   - 完成后做 WAL checkpoint 并输出逐表行数校验，行数不一致以非零码退出
package main

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"time"

	"rental-server/config"
	"rental-server/database"
	"rental-server/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// allModels 迁移顺序即写入顺序，父表在前以满足目标库外键约束。
var allModels = []interface{}{
	&models.User{},
	&models.Building{},
	&models.BuildingLandlord{},
	&models.BuildingRenewal{},
	&models.Room{},
	&models.RoomMedia{},
	&models.Tenant{},
	&models.RentalContract{},
	&models.Bill{},
	&models.Shareholder{},
	&models.Dividend{},
	&models.Task{},
	&models.Setting{},
	&models.RecruitSubmission{},
	&models.AuditLog{},
	&models.PageView{},
}

const batchSize = 500

func main() {
	out := flag.String("out", "./storage/export.db", "SQLite 目标文件路径")
	force := flag.Bool("force", false, "目标文件已存在时强制覆盖")
	flag.Parse()

	cfg := config.Load()

	// 源库：MySQL，连接参数与 db_driver 取值无关
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	src, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("连接 MySQL 源库失败: %v", err))
	}

	if _, err := os.Stat(*out); err == nil {
		if !*force {
			panic(fmt.Sprintf("目标文件已存在: %s（加 -force 覆盖）", *out))
		}
		for _, suffix := range []string{"", "-wal", "-shm"} {
			os.Remove(*out + suffix)
		}
	}

	// 目标库复用 database 包的 SQLite 初始化（WAL、PRAGMA、连接池），仅替换路径
	tcfg := *cfg
	tcfg.DBDriver = config.DriverSQLite
	tcfg.DBPath = *out
	dst, err := database.Open(&tcfg)
	if err != nil {
		panic(fmt.Sprintf("初始化 SQLite 目标库失败: %v", err))
	}
	if err := models.AutoMigrate(dst); err != nil {
		panic(fmt.Sprintf("目标库建表失败: %v", err))
	}

	fmt.Printf("开始迁移 -> %s\n", *out)
	start := time.Now()
	totalSrc, totalDst, mismatch := int64(0), int64(0), false

	for _, m := range allModels {
		name := tableName(src, m)

		var srcCount int64
		if err := src.Model(m).Unscoped().Count(&srcCount).Error; err != nil {
			panic(fmt.Sprintf("统计源表 %s 失败: %v", name, err))
		}

		if srcCount > 0 {
			// Unscoped 连软删除记录一起搬；按主键分批读，Omit 关联避免 GORM
			// 把模型里的 Room/Tenant 等零值关联字段误当成新数据写进目标库
			dest := reflect.New(reflect.SliceOf(reflect.TypeOf(m).Elem())).Interface()
			err := src.Model(m).Unscoped().FindInBatches(dest, batchSize, func(tx *gorm.DB, batch int) error {
				if err := dst.Omit(clause.Associations).CreateInBatches(dest, batchSize).Error; err != nil {
					return fmt.Errorf("写入第 %d 批失败: %w", batch, err)
				}
				return nil
			}).Error
			if err != nil {
				panic(fmt.Sprintf("迁移表 %s 失败: %v", name, err))
			}
		}

		var dstCount int64
		if err := dst.Model(m).Unscoped().Count(&dstCount).Error; err != nil {
			panic(fmt.Sprintf("统计目标表 %s 失败: %v", name, err))
		}

		mark := "ok"
		if srcCount != dstCount {
			mark = "行数不一致!"
			mismatch = true
		}
		fmt.Printf("  %-22s %6d -> %6d  %s\n", name, srcCount, dstCount, mark)
		totalSrc += srcCount
		totalDst += dstCount
	}

	// WAL 收尾：把 -wal 日志并回主文件，输出单个可直接拷贝部署的 .db
	if err := dst.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		fmt.Printf("警告: WAL checkpoint 失败: %v\n", err)
	}

	fmt.Printf("迁移完成：%d -> %d 行，耗时 %s\n", totalSrc, totalDst, time.Since(start).Round(time.Millisecond))
	if mismatch || totalSrc != totalDst {
		fmt.Println("警告：源/目标行数不一致，请检查上方标记后重跑")
		os.Exit(1)
	}
	fmt.Printf("切换方式：config.json 中设 \"db_driver\": %q 并将 \"db_path\" 指向该文件（或把它复制为服务器上的 ./storage/rental.db）\n",
		config.DriverSQLite)
}

// tableName 解析模型的实际表名，用于日志输出。
func tableName(db *gorm.DB, model interface{}) string {
	stmt := &gorm.Statement{DB: db}
	if err := stmt.Parse(model); err != nil {
		return "unknown"
	}
	return stmt.Schema.Table
}
