// setadmin 直接修改数据库中超级管理员的账号密码（不经 HTTP 接口），部署服务器上就地换凭据。
//
// 流程与校验：
//  1. 读取 config.yaml（或 CONFIG_PATH 指定的文件），按 database.driver 连接 MySQL 或 SQLite
//  2. 按原账号查超级管理员（role=super_admin 且未删除），bcrypt 校验原密码
//  3. 校验新密码与确认密码一致、新账号未被其他用户占用（含软删除占位，与唯一索引一致）
//  4. 把新账号与 bcrypt 新哈希写库，立即生效，无需重启后端
//
// 注意：
//   - ★ 改完务必同步 config.yaml 的 admin.username / admin.password（或 ADMIN_USERNAME /
//     ADMIN_PASSWORD 环境变量）：后端每次启动都会把超管账号密码与配置对齐（有则改、无则建），
//     不同步则下次重启会被改回配置里的旧值
//   - 已签发的登录令牌在有效期内仍然可用，不会因改密立即失效
//   - 必须在 server 目录下运行（config.yaml 从当前工作目录读取）
//
// 运行方式（在 server 目录下）：
//
//	go run ./scripts/setadmin <原账号> <原密码> <新账号> <新密码> <确认密码>
package main

import (
	"flag"
	"fmt"
	"os"

	"rental-server/config"
	"rental-server/database"
	"rental-server/models"

	"golang.org/x/crypto/bcrypt"
)

func usage() {
	fmt.Fprint(os.Stderr, `setadmin 直接修改数据库中超级管理员的账号密码（支持 sqlite / mysql 双后端）。

用法（在 server 目录下）:
  go run ./scripts/setadmin <原账号> <原密码> <新账号> <新密码> <确认密码>

示例:
  go run ./scripts/setadmin root root admin NewPass@123 NewPass@123

改完务必同步 config.yaml 的 admin.username / admin.password 为新值，
否则下次重启后端会把超级管理员的账号密码对齐回配置文件里的旧值。
配置读取与后端一致（config.yaml / CONFIG_PATH / ADMIN_USERNAME / ADMIN_PASSWORD 环境变量）。
`)
}

func die(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) != 5 {
		usage()
		os.Exit(2)
	}
	oldUsername, oldPassword, newUsername, newPassword, newConfirm :=
		args[0], args[1], args[2], args[3], args[4]

	if oldUsername == "" || oldPassword == "" || newUsername == "" || newPassword == "" {
		die("账号与密码均不能为空")
	}
	if newPassword != newConfirm {
		die("新密码与确认密码不一致")
	}

	cfg := config.Load()
	db, err := database.Open(cfg)
	if err != nil {
		panic(err)
	}

	// 1. 原账号必须是现存超级管理员（gorm 自动排除软删除行）
	var admin models.User
	if err := db.Where("username = ? AND role = ?", oldUsername, "super_admin").First(&admin).Error; err != nil {
		die("原账号 %q 不存在或不是超级管理员", oldUsername)
	}

	// 2. bcrypt 校验原密码
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(oldPassword)); err != nil {
		die("原密码错误")
	}
	if !admin.IsActive {
		fmt.Println("警告: 该账号当前处于停用状态（is_active=false），改完密码仍无法登录，请先在管理后台启用")
	}

	// 3. 新账号查重：Unscoped 含软删除行，与 username 唯一索引的实际占用一致
	if newUsername != admin.Username {
		var conflict models.User
		if err := db.Unscoped().Where("username = ?", newUsername).First(&conflict).Error; err == nil && conflict.ID != admin.ID {
			die("新账号 %q 已被其他用户占用", newUsername)
		}
	}

	// 4. 写库（bcrypt 与后端登录校验同一算法/成本）
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		die("新密码加密失败: %v", err)
	}
	if err := db.Model(&admin).Updates(map[string]interface{}{
		"username":      newUsername,
		"password_hash": string(hash),
	}).Error; err != nil {
		die("写入数据库失败: %v", err)
	}

	fmt.Printf("超级管理员已更新: %s → %s（数据库已生效，无需重启）\n", oldUsername, newUsername)
	fmt.Println("★ 请同步修改 config.yaml 的 admin.username / admin.password 为新值，")
	fmt.Println("  否则下次重启后端会把超级管理员的账号密码对齐回配置文件里的旧值。")
}
