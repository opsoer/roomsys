package envcheck

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"rental-server/config"
	"rental-server/migrations"
	"rental-server/qiniu"

	"gorm.io/gorm"
)

// checksFor 组装启动自检项。db 为 nil 时跳过数据库检查（测试用）。
func checksFor(cfg *config.Config, db *gorm.DB, version string) []Check {
	checks := []Check{
		{Name: "基础信息", Run: func(context.Context) (string, error) { return baseInfo(cfg, version) }},
		{Name: "上传目录", Run: func(ctx context.Context) (string, error) { return checkUploadDir(ctx, cfg.Server.UploadDir) }},
		{Name: "对象存储", Run: func(ctx context.Context) (string, error) { return checkQiniu(cfg) }},
		{Name: "CDN 域名", Run: func(ctx context.Context) (string, error) { return checkCDN(ctx, cfg) }},
		{Name: "媒体服务", Run: func(context.Context) (string, error) { return checkMedia(cfg) }},
		{Name: "前端产物", Run: func(context.Context) (string, error) { return checkWebDist(cfg.Server.WebDistDir) }},
		{Name: "监听端口", Run: func(context.Context) (string, error) { return checkPort(cfg.Server.Port) }},
		{Name: "定时任务", Run: func(context.Context) (string, error) { return checkScheduler(cfg) }},
		{Name: "安全", Run: func(context.Context) (string, error) { return checkSecurity(cfg) }},
	}
	if db != nil {
		checks = append([]Check{{
			Name: "数据库",
			Run:  func(ctx context.Context) (string, error) { return checkDatabase(ctx, cfg, db) },
		}}, checks...)
	}
	return checks
}

// baseInfo 打印版本、部署模式、端口、日志级别与关键目录等基础信息。
func baseInfo(cfg *config.Config, version string) (string, error) {
	src := os.Getenv("CONFIG_PATH")
	if src == "" {
		src = "config.yaml"
	}
	return fmt.Sprintf("version=%s mode=%s port=%s log=%s config=%s upload=%s web=%s",
		version, cfg.Server.Mode, cfg.Server.Port, cfg.Server.LogLevel, src,
		cfg.Server.UploadDir, cfg.Server.WebDistDir), nil
}

// checkDatabase 确认数据库可连接，并打印驱动、位置与迁移状态。
func checkDatabase(ctx context.Context, cfg *config.Config, db *gorm.DB) (string, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return "", fmt.Errorf("获取底层连接失败: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return "", fmt.Errorf("ping 失败: %w", err)
	}

	switch db.Dialector.Name() {
	case "sqlite":
		path := cfg.Database.SQLite.Path
		var size int64
		if info, statErr := os.Stat(path); statErr == nil {
			size = info.Size()
		}
		var mode string
		if err := db.Raw("PRAGMA journal_mode").Scan(&mode).Error; err != nil {
			mode = "未知"
		}
		applied, pending, err := migrations.Status(db)
		if err != nil {
			return "", fmt.Errorf("读取迁移状态失败: %w", err)
		}
		detail := fmt.Sprintf("sqlite %s（journal_mode=%s，%.2f MB，迁移已应用 %d 个，待执行 %d 个）",
			path, mode, float64(size)/1024/1024, len(applied), len(pending))
		if len(pending) > 0 {
			return detail, Warnf("存在 %d 个待执行迁移", len(pending))
		}
		return detail, nil
	case "mysql":
		var ver string
		if err := db.Raw("SELECT VERSION()").Scan(&ver).Error; err != nil {
			return "", fmt.Errorf("查询版本失败: %w", err)
		}
		m := cfg.Database.MySQL
		return fmt.Sprintf("mysql %s:%s/%s（版本 %s）", m.Host, m.Port, m.Name, ver), nil
	default:
		return "", fmt.Errorf("未知数据库驱动: %s", db.Dialector.Name())
	}
}

// checkUploadDir 验证上传目录可创建、可写。
func checkUploadDir(_ context.Context, dir string) (string, error) {
	if dir == "" {
		return "", errors.New("upload_dir 未配置")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("创建失败: %w", err)
	}
	f, err := os.CreateTemp(dir, ".envcheck-*")
	if err != nil {
		return "", fmt.Errorf("写入失败: %w", err)
	}
	name := f.Name()
	if _, err := f.WriteString("ok"); err != nil {
		f.Close()
		os.Remove(name)
		return "", fmt.Errorf("写入失败: %w", err)
	}
	f.Close()
	if err := os.Remove(name); err != nil {
		return "", fmt.Errorf("清理失败: %w", err)
	}
	return fmt.Sprintf("%s 可写", dir), nil
}

// checkQiniu 验证七牛对象存储连通性：未配置记 SKIP，已配置则做一次只读列举。
func checkQiniu(cfg *config.Config) (string, error) {
	q := cfg.Storage.Qiniu
	if !qiniu.Enabled(cfg) {
		return "未配置七牛对象存储（access_key/secret_key/bucket 为空），媒体使用本地存储", ErrSkip
	}
	keys, err := qiniu.ListKeys(cfg, "healthcheck/")
	if err != nil {
		return "", fmt.Errorf("bucket=%s zone=%s 连接失败: %w", q.Bucket, q.Zone, err)
	}
	return fmt.Sprintf("七牛 bucket=%s zone=%s 连接正常（healthcheck/ 下 %d 个对象）",
		q.Bucket, q.Zone, len(keys)), nil
}

// checkCDN 验证 CDN 域名可达性：404/403 等响应也算可达，只有网络层失败才算 ERROR。
func checkCDN(ctx context.Context, cfg *config.Config) (string, error) {
	q := cfg.Storage.Qiniu
	if q.Domain == "" {
		return "未配置 CDN 域名，首次媒体请求时自动从 bucket 发现", ErrSkip
	}
	scheme := "http"
	if q.UseHTTPS {
		scheme = "https"
	}
	u := scheme + "://" + q.Domain + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RentalServer/1.0)")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s 不可达: %w", u, err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	detail := fmt.Sprintf("%s 可达（HTTP %d）", u, resp.StatusCode)
	if !q.UseHTTPS && strings.Contains(q.Domain, "clouddn.com") {
		return detail, Warnf("使用 HTTP 测试域名 %s，测试域名约 30 天到期，正式上线请换已备案的 HTTPS 域名", q.Domain)
	}
	return detail, nil
}

// checkMedia 打印媒体访问模式与转码配置，并对高风险组合给出警告。
func checkMedia(cfg *config.Config) (string, error) {
	q := cfg.Storage.Qiniu
	if !qiniu.Enabled(cfg) {
		return "本地存储模式（媒体文件由 upload_dir 托管）", nil
	}
	mode := "代理模式"
	if !q.ProxyMode {
		mode = "直连模式"
	}
	pfop := "关闭"
	if q.PfopEnable {
		pfop = "开启"
	}
	detail := fmt.Sprintf("七牛 %s，视频转码 %s", mode, pfop)

	var warns []string
	if !q.ProxyMode && !q.UseHTTPS {
		warns = append(warns, "直连模式 + HTTP 域名：微信内置浏览器会拦截媒体资源，建议开启代理模式或换 HTTPS 域名")
	}
	if q.PfopEnable {
		if q.PfopCallbackSecret == "" {
			warns = append(warns, "已开启转码但 pfop_callback_secret 为空，七牛回调会被拒绝")
		}
		if q.PfopCallbackURL == "" {
			warns = append(warns, "未配置 pfop_callback_url，将用上传请求的 Host 推导回调地址")
		}
	}
	if len(warns) > 0 {
		return detail, Warnf("%s", strings.Join(warns, "；"))
	}
	return detail, nil
}

// checkWebDist 确认前端构建产物存在（缺失会导致页面白屏）。
func checkWebDist(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("web_dist_dir 未配置")
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		return "", fmt.Errorf("index.html 不存在: %s", index)
	}
	assets := filepath.Join(dir, "assets")
	info, err := os.Stat(assets)
	if err != nil || !info.IsDir() {
		return fmt.Sprintf("index.html 存在，但 assets 目录缺失：%s", assets), Warnf("页面可能白屏或样式缺失")
	}
	return fmt.Sprintf("%s（index.html + assets 就绪）", dir), nil
}

// checkPort 提前探测 HTTP 端口是否可监听。
func checkPort(port string) (string, error) {
	if port == "" {
		return "", errors.New("server.port 未配置")
	}
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return "", fmt.Errorf("端口 %s 不可用: %w", port, err)
	}
	_ = ln.Close()
	return fmt.Sprintf(":%s 可监听", port), nil
}

// checkScheduler 打印已启用的定时任务，并检查备份等任务的外部依赖。
func checkScheduler(cfg *config.Config) (string, error) {
	type job struct {
		name string
		c    config.JobConfig
	}
	jobs := []job{
		{"rent_bills", cfg.Scheduler.RentBills},
		{"building_expiry", cfg.Scheduler.BuildingExpiry},
		{"contract_expiry", cfg.Scheduler.ContractExpiry},
		{"soft_delete_cleanup", cfg.Scheduler.SoftDeleteCleanup},
		{"token_cleanup", cfg.Scheduler.TokenCleanup},
		{"db_backup", cfg.Scheduler.DBBackup},
		{"bill_evidence_gc", cfg.Scheduler.BillEvidenceGC},
		{"media_gc", cfg.Scheduler.MediaGC},
	}
	enabled := make([]string, 0, len(jobs))
	for _, j := range jobs {
		if j.c.Enabled {
			enabled = append(enabled, j.name)
		}
	}
	detail := fmt.Sprintf("已启用 %d 项：%s", len(enabled), strings.Join(enabled, "、"))

	var warns []string
	if cfg.Scheduler.DBBackup.Enabled && !qiniu.Enabled(cfg) {
		warns = append(warns, "db_backup 已启用但未配置七牛对象存储，备份会失败")
	}
	if cfg.Scheduler.DBBackup.Enabled && cfg.Database.Driver == config.DriverMySQL {
		if _, err := exec.LookPath("mysqldump"); err != nil {
			warns = append(warns, "MySQL 备份依赖 mysqldump，当前 PATH 中未找到")
		}
	}
	if len(warns) > 0 {
		return detail, Warnf("%s", strings.Join(warns, "；"))
	}
	return detail, nil
}

// checkSecurity 对默认凭据、弱密钥与调试端口暴露等做风险提示。
func checkSecurity(cfg *config.Config) (string, error) {
	var warns []string
	if cfg.Admin.Username == "root" && cfg.Admin.Password == "root" {
		warns = append(warns, "超级管理员为默认 root/root，生产环境请修改")
	}
	if len(cfg.Server.JWTSecret) < 16 {
		warns = append(warns, "jwt_secret 长度不足 16，建议使用随机长密钥")
	}
	if cfg.Server.PprofAddr != "" && cfg.Server.PprofAllowRemote {
		warns = append(warns, "pprof 允许绑定非回环地址，存在无鉴权暴露风险")
	}
	if len(warns) > 0 {
		return "存在风险项", Warnf("%s", strings.Join(warns, "；"))
	}
	return "未发现明显风险", nil
}
