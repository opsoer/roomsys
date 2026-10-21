// migrate 独立于后端进程执行数据库迁移：按编号应用尚未执行的结构变更 + 确保 root 账号存在。
//
// 使用场景：
//   - 部署前显式升级数据库结构（不依赖后端启动时自动迁移）
//   - 查看当前数据库已应用/待应用的迁移版本
//   - 新环境初始化数据库
//
// 基本原理：
//  1. 读取 config.yaml（或 CONFIG_PATH 指定的文件），按 database.driver 连接 MySQL 或 SQLite
//  2. 调用 migrations.Apply：按编号顺序执行尚未应用的步骤，并把版本记入 schema_migrations 表
//     （结构变更的「台账」，可查可审计；详见 migrations 包说明）
//  3. 若 username=root 的账号不存在则创建（root/root，super_admin）；已存在则不动
//
// 常用参数：
//
//	-status     只打印迁移状态，不执行
//	-rollback=N 回退到版本 N（不含 N）
//
// 注意：
//   - 必须在 server 目录下运行（config.yaml 从当前工作目录读取）
//   - 后端启动时也会执行同一套迁移逻辑，两者等价，重复执行是幂等的
//
// 运行方式（在 server 目录下）：go run ./scripts/migrate
package main

import (
	"flag"
	"fmt"

	"rental-server/config"
	"rental-server/database"
	"rental-server/migrations"
	"rental-server/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	statusOnly := flag.Bool("status", false, "只显示迁移状态，不执行迁移")
	rollback := flag.Int("rollback", -1, "回退到指定版本（不含该版本），如 -rollback=1")
	flag.Parse()

	cfg := config.Load()
	db, err := database.Open(cfg)
	if err != nil {
		panic(err)
	}

	if *statusOnly {
		printStatus(db)
		return
	}

	if *rollback >= 0 {
		rolled, err := migrations.Rollback(db, *rollback)
		if err != nil {
			panic(err)
		}
		fmt.Printf("已回退 %d 个迁移版本\n", len(rolled))
		for _, v := range rolled {
			fmt.Printf("  - %03d\n", v)
		}
		return
	}

	applied, err := migrations.Apply(db)
	if err != nil {
		panic(err)
	}
	if len(applied) == 0 {
		fmt.Println("数据库结构已是最新，无需迁移")
	} else {
		fmt.Printf("已应用 %d 个迁移版本\n", len(applied))
		for _, v := range applied {
			fmt.Printf("  + %03d\n", v)
		}
	}

	seedAdmin(db)
}

// printStatus 打印已应用与待应用的迁移版本。
func printStatus(db *gorm.DB) {
	applied, pending, err := migrations.Status(db)
	if err != nil {
		panic(err)
	}
	fmt.Println("已应用的迁移版本:")
	for _, v := range applied {
		fmt.Printf("  ✓ %03d\n", v)
	}
	if len(applied) == 0 {
		fmt.Println("  (无)")
	}
	fmt.Println("待执行的迁移步骤:")
	if len(pending) == 0 {
		fmt.Println("  (无)")
	}
	for _, s := range pending {
		fmt.Printf("  → %s\n", s.Label())
	}
}

func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "root").First(&admin)
	password := "root"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}

	if result.Error != nil {
		admin = models.User{
			Username:     "root",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		db.Create(&admin)
		fmt.Printf("已创建默认超级管理员: root / %s\n", password)
	}
}
