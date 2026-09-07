// migrate 独立于后端进程执行数据库初始化：建表迁移 + 确保 root 账号存在。
//
// 使用场景：
//   - 部署新环境时手动初始化数据库（不想靠首次启动后端来建表）
//   - 改了 models 后想在不启动完整后端的情况下先把表结构升上去
//   - CI/脚本化部署里作为独立的迁移步骤
//
// 基本原理：
//  1. 读取 config.json（或 CONFIG_PATH 指定的文件），按 db_driver 连接 MySQL 或 SQLite
//  2. 调 models.AutoMigrate 建表/补列/建索引 —— 与后端启动时执行的是同一份迁移逻辑，
//     所以跑不跑本脚本，最终表结构一致
//  3. 若 username=root 的账号不存在则创建（root/root，super_admin）；已存在则不动
//
// 注意：
//   - 必须在 server 目录下运行（config.json 从当前工作目录读取）
//   - 后端每次启动都会把 root 密码重置为 root，本脚本不会
//
// 运行方式（在 server 目录下）：go run ./scripts/migrate
package main

import (
	"fmt"

	"rental-server/config"
	"rental-server/database"
	"rental-server/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func main() {
	cfg := config.Load()
	db, err := database.Open(cfg)
	if err != nil {
		panic(err)
	}

	if err := models.AutoMigrate(db); err != nil {
		panic(err)
	}
	fmt.Println("数据库迁移完成")

	seedAdmin(db)
}

func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "root").First(&admin)
	if result.Error != nil {
		password := "root"
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		admin = models.User{
			Username:     "root",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		db.Create(&admin)
		fmt.Printf("已创建默认超级管理员: root / %s\n", password)
	}
}
