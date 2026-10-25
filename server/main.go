// main 是应用程序入口，负责初始化配置、数据库、路由并启动 HTTP 服务。
package main

import (
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rental-server/audit"
	"rental-server/config"
	"rental-server/database"
	"rental-server/handlers"
	"rental-server/logger"
	"rental-server/middleware"
	"rental-server/migrations"
	"rental-server/models"
	"rental-server/routes"
	"rental-server/scheduler"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Version 编译期注入的版本号（CI 构建时写入 commit sha），用于确认线上跑的是哪一版。
var Version = "dev"

// main 是程序入口：加载配置、初始化日志、连接数据库、设置定时任务和路由。
func main() {
	cfg := config.Load()

	logger.Init(logger.Config{
		Level: cfg.Server.LogLevel,
		Dir:   cfg.Server.LogDir,
	})
	logger.Log.Info().Str("level", cfg.Server.LogLevel).Str("dir", cfg.Server.LogDir).Msg("日志系统初始化完成")

	db, err := database.Open(cfg)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库连接失败")
	}
	logger.Log.Info().Str("driver", cfg.Database.Driver).Msg("数据库连接成功")

	// 审计模块需要数据库句柄：写操作审计既写日志文件，也落库到 audit_logs 表
	audit.Init(db)

	if os.Getenv("RESET_DATABASE") == "true" {
		logger.Log.Warn().Msg("检测到 RESET_DATABASE=true，将删除所有表并重建")
		resetDatabase(db)
	}

	// 数据库结构迁移：按编号顺序执行尚未应用的版本（台账表 schema_migrations）
	migrated, err := migrations.Apply(db)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("数据库迁移失败")
	}
	if len(migrated) > 0 {
		logger.Log.Info().Ints("versions", migrated).Msg("数据库结构已更新")
	} else {
		logger.Log.Info().Msg("数据库结构已是最新")
	}

	seedAdmin(db)

	// 查看房东电话的免费额度来自配置；必须在 HTTP 服务开始接收请求前初始化
	utils.InitRevealQuota(cfg.Server.RevealFreePerDay)

	// 定时任务注册表：定义/周期来自 config.yaml scheduler 段，统一调度与手动执行共用一份注册
	jobs := scheduler.NewRegistry()
	jobs.Add(scheduler.NewJob("rent_bills", "月度租金账单生成", cfg.Scheduler.RentBills, true,
		func() string { return handlers.AutoCreateMonthlyRentBills(db) }))
	jobs.Add(scheduler.NewJob("building_expiry", "公寓到期检查", cfg.Scheduler.BuildingExpiry, true,
		func() string { return scheduler.CheckExpiredBuildings(db) }))
	jobs.Add(scheduler.NewJob("contract_expiry", "到期合同检查", cfg.Scheduler.ContractExpiry, true,
		func() string { return scheduler.AutoCheckExpiringContracts(db) }))
	jobs.Add(scheduler.NewJob("soft_delete_cleanup", "软删除清理", cfg.Scheduler.SoftDeleteCleanup, false,
		func() string { return scheduler.AutoCleanupData(db) }))
	jobs.Add(scheduler.NewJob("token_cleanup", "吊销令牌清理", cfg.Scheduler.TokenCleanup, false,
		utils.CleanupRevokedTokens))
	jobs.Add(scheduler.NewJob("db_backup", "数据库备份到对象存储", cfg.Scheduler.DBBackup, false,
		func() string { return scheduler.BackupDatabase(db, cfg) }))
	jobs.Start()

	if err := os.MkdirAll(cfg.Server.UploadDir, 0750); err != nil {
		logger.Log.Fatal().Err(err).Str("dir", cfg.Server.UploadDir).Msg("创建上传目录失败")
	}
	logger.Log.Info().Str("dir", cfg.Server.UploadDir).Msg("上传目录就绪")

	ensureFFmpegCore(cfg.Server.UploadDir)

	if gin.Mode() != gin.ReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.MaxMultipartMemory = 200 << 20 // 200MB

	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigins := []string{
			"http://localhost:5173",
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:5173",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
		if extraOrigins := os.Getenv("CORS_ORIGINS"); extraOrigins != "" {
			for _, o := range splitStr(extraOrigins, ",") {
				allowedOrigins = append(allowedOrigins, o)
			}
		}
		allowed := false
		for _, o := range allowedOrigins {
			if origin == o {
				allowed = true
				break
			}
		}
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Requested-With, X-CSRF-Token")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	r.Use(func(c *gin.Context) {
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob: https:; media-src 'self' https:; font-src 'self' data:; worker-src 'self' blob:; connect-src 'self' https://*.qiniup.com https://*.qiniudn.com")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if gin.Mode() == gin.ReleaseMode {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	})

	r.Use(gzipMiddleware())
	r.Use(cacheControlMiddleware())

	r.Use(middleware.RateLimitMiddleware())

	// 初始化统计写入器
	utils.InitStatsWriter(db)

	routes.Setup(r, db, cfg, jobs)

	// 按需启动 pprof 调试端口（默认关闭）：性能火焰图与内存/goroutine 分析入口，
	// 开启方式与安全约束见《性能火焰图使用指南.md》
	if pprofAddr := os.Getenv("PPROF_ADDR"); pprofAddr != "" {
		startPprof(pprofAddr)
	}

	logger.Log.Info().Str("port", cfg.Server.Port).Str("version", Version).Msg("服务启动")
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		logger.Log.Fatal().Err(err).Msg("HTTP 服务启动失败")
	}
}

// startPprof 按需启动 pprof 调试端口（性能火焰图与内存/goroutine 分析，默认不启动）。
//
// 开启方式：设置环境变量 PPROF_ADDR，例如 PPROF_ADDR=127.0.0.1:6060。
// 安全约束：pprof 端点无鉴权（可读取堆内存画像、可触发全局 GC 停顿），因此只允许
// 绑定回环地址，供本机直连或 SSH 隧道访问；确需绑定外部地址（如容器网络）时，
// 必须再显式设置 PPROF_ALLOW_REMOTE=true，且只应在受控内网中使用。
func startPprof(addr string) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		logger.Log.Warn().Str("addr", addr).Err(err).Msg("PPROF_ADDR 格式非法（应为 host:port，如 127.0.0.1:6060），pprof 未启动")
		return
	}
	// host 为空（如 ":6060"）意味着监听所有网卡，与绑定外部地址同等对待，不放行。
	// 仅按 IP 字面量判断回环，避免 "127.0.0.1.example.com" 这类域名通过前缀检查后解析到外部地址
	loopback := strings.EqualFold(host, "localhost")
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		loopback = true
	}
	if !loopback && os.Getenv("PPROF_ALLOW_REMOTE") != "true" {
		logger.Log.Warn().Str("addr", addr).Msg("PPROF_ADDR 拒绝非回环地址：pprof 无鉴权，只能绑定 127.0.0.1/localhost（远程访问走 SSH 隧道）；确需外部绑定时设置 PPROF_ALLOW_REMOTE=true 自担风险")
		return
	}
	go func() {
		// 独立端口 + 独立 mux 只承载 pprof，避免 DefaultServeMux 上其他包注册的处理器被一并暴露，
		// 也与业务端口的 gin 中间件链（限流/CORS/gzip）互不干扰
		mux := http.NewServeMux()
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		srv := &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
			// 长采样（profile?seconds=90）在采样结束后才开始写响应，写超时需覆盖采样时长
			WriteTimeout: 120 * time.Second,
		}
		logger.Log.Info().Str("addr", addr).Msg("pprof 调试端口已启动，访问 http://" + addr + "/debug/pprof/（用完即关：去掉 PPROF_ADDR 并重启）")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Warn().Err(err).Msg("pprof 调试端口退出")
		}
	}()
}

// requestLogger 返回 Gin 中间件，记录每个 HTTP 请求的方法、路径、状态码和耗时。
//
// 分级策略（写操作审计由 audit 包单独记录，这里只做请求轨迹）：
//   - 2xx/3xx：查询与读请求，默认不输出（降为 Debug），避免访问日志淹没关键信息；
//   - 4xx：客户端错误，记 Warn（越权、参数错误、限流等）；
//   - 5xx：服务端错误，记 Error。
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		lvl := zerolog.DebugLevel
		if status >= 500 {
			lvl = zerolog.ErrorLevel
		} else if status >= 400 {
			lvl = zerolog.WarnLevel
		}

		logger.Log.WithLevel(lvl).
			Str("method", method).
			Str("path", path).
			Str("query", query).
			Int("status", status).
			Str("ip", clientIP).
			Dur("latency", latency).
			Msg("")
	}
}

// resetDatabase 删除所有表，供 AutoMigrate 重建。
func resetDatabase(db *gorm.DB) {
	tables := []interface{}{
		&models.AuditLog{},
		&models.RecruitSubmission{},
		&models.Setting{},
		&models.Task{},
		&models.Dividend{},
		&models.Shareholder{},
		&models.Bill{},
		&models.RentalContract{},
		&models.Tenant{},
		&models.RoomMedia{},
		&models.Room{},
		&models.BuildingLandlord{},
		&models.BuildingRenewal{},
		&models.Building{},
		&models.User{},
		&models.PageView{},
		&models.LocationCustom{},
		&models.LocationRename{},
	}
	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			logger.Log.Warn().Err(err).Msg("删除表失败")
		}
	}
	logger.Log.Info().Msg("所有表已删除，等待 AutoMigrate 重建")
}

// seedAdmin 检查是否存在 root 用户，如不存在则创建，否则重置密码。
func seedAdmin(db *gorm.DB) {
	var admin models.User
	result := db.Where("username = ?", "root").First(&admin)
	password := "root"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Fatal().Err(err).Msg("管理员密码加密失败")
		return
	}

	if result.Error != nil {
		admin = models.User{
			Username:     "root",
			PasswordHash: string(hash),
			Role:         "super_admin",
		}
		if err := db.Create(&admin).Error; err != nil {
			logger.Log.Fatal().Err(err).Msg("创建默认管理员失败")
			return
		}
		logger.Log.Info().Msg("已创建默认超级管理员: root / root")
	} else {
		db.Model(&admin).Updates(map[string]interface{}{
			"password_hash": string(hash),
			"role":          "super_admin",
		})
		logger.Log.Info().Msg("已重置超级管理员密码: root / root")
	}
}

// ensureFFmpegCore 确保 FFmpeg WASM 核心文件已下载到上传目录。
func ensureFFmpegCore(uploadDir string) {
	dir := filepath.Join(uploadDir, "ffmpeg")
	if err := os.MkdirAll(dir, 0750); err != nil {
		logger.Log.Warn().Err(err).Msg("创建 ffmpeg 目录失败")
		return
	}

	baseURL := "https://cdn.jsdelivr.net/npm/@ffmpeg/core@0.12.10/dist/esm"
	files := []string{"ffmpeg-core.js", "ffmpeg-core.wasm"}

	for _, f := range files {
		path := filepath.Join(dir, f)
		if _, err := os.Stat(path); err == nil {
			logger.Log.Info().Str("file", f).Msg("FFmpeg core 文件已存在，跳过下载")
			continue
		}

		url := baseURL + "/" + f
		logger.Log.Info().Str("file", f).Msg("正在下载 FFmpeg core 文件")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Get(url)
		if err != nil {
			logger.Log.Warn().Err(err).Str("url", url).Msg("下载 FFmpeg core 失败")
			continue
		}

		out, err := os.Create(path)
		if err != nil {
			logger.Log.Warn().Err(err).Msg("创建 ffmpeg 文件失败")
			resp.Body.Close()
			continue
		}

		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			logger.Log.Warn().Err(err).Msg("写入 ffmpeg 文件失败")
			os.Remove(path)
		} else {
			logger.Log.Info().Str("file", f).Msg("FFmpeg core 文件下载完成")
		}
	}
}

// gzipWriter 包装 gin.ResponseWriter，用 gzip 压缩响应体。
type gzipWriter struct {
	gin.ResponseWriter
	writer  *gzip.Writer
	written int64
}

// Write 使用 gzip.Writer 压缩后写入响应，同时记录原始写入字节数。
// 不能依赖内嵌 ResponseWriter 的 Size()：经本包装后它无法反映 handler 是否实际写出内容。
func (g *gzipWriter) Write(data []byte) (int, error) {
	g.written += int64(len(data))
	return g.writer.Write(data)
}

// skipGzipPrefixes 不需要 gzip 压缩的路径前缀（媒体文件和 FFmpeg wasm 已压缩过）
var skipGzipPrefixes = []string{"/api/media/", "/api/ffmpeg/"}

// minGzipLength 小于该长度的响应不压缩：gzip 头部 + 动态哈夫曼表约 20 字节，
// 压缩几十字节的小 JSON 不仅不省流量，反而会变大且白白消耗 CPU。
const minGzipLength = 256

// gzipWriterPool 复用 gzip.Writer。gzip.NewWriterLevel 每次都会分配约 400KB
// 的 deflate 哈希表与窗口（火焰图实测占全部堆分配的 95%），按请求新建会把
// 大量内存压力转移给 GC；Reset 只重置状态、保留缓冲区，可把分配降到接近 0。
var gzipWriterPool = sync.Pool{
	New: func() any {
		gz, err := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		if err != nil {
			return nil
		}
		return gz
	},
}

// bufferedGzipWriter 先把响应缓存到内存，写完后按实际长度决定是否压缩：
// 小响应直接明文写出，大响应再灌入复用的 gzip.Writer，避免产生「压缩后反而更大」的响应。
type bufferedGzipWriter struct {
	gin.ResponseWriter
	body []byte
	code int
}

// WriteHeader 只记录状态码，真正写头推迟到 flush，避免提前锁死响应。
func (w *bufferedGzipWriter) WriteHeader(code int) {
	w.code = code
}

// Write 只缓存响应体，不直接写给客户端。
func (w *bufferedGzipWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

// WriteString 与 Write 等价，供 gin 的字符串渲染路径使用。
func (w *bufferedGzipWriter) WriteString(s string) (int, error) {
	w.body = append(w.body, s...)
	return len(s), nil
}

// gzipMiddleware 返回 Gin 中间件，对符合条件的请求启用 gzip 压缩。
func gzipMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}
		// 图片/视频/FFmpeg wasm 已经压缩过，跳过
		for _, p := range skipGzipPrefixes {
			if strings.HasPrefix(path, p) {
				c.Next()
				return
			}
		}
		origin := c.Writer
		buf := &bufferedGzipWriter{ResponseWriter: origin}
		c.Writer = buf
		c.Next()

		// 先把压缩决策需要的信息准备好，再恢复原始写入器，避免响应头已被隐式写出
		status := buf.code
		if status == 0 {
			status = origin.Status()
		}
		if status == 0 {
			status = http.StatusOK
		}
		// 仅压缩成功响应：错误响应通常很小，且需要让前端按原始长度解析错误体
		compressible := status == http.StatusOK && len(buf.body) >= minGzipLength
		// 无论客户端是否声明支持 gzip，都要声明 Vary，避免中间缓存串用不同编码的响应
		c.Header("Vary", "Accept-Encoding")

		if !compressible {
			origin.WriteHeader(status)
			if len(buf.body) > 0 {
				_, _ = origin.Write(buf.body)
			}
			return
		}

		gz, _ := gzipWriterPool.Get().(*gzip.Writer)
		if gz == nil {
			origin.WriteHeader(status)
			_, _ = origin.Write(buf.body)
			return
		}
		gz.Reset(origin)
		// 压缩后才设置头：之前有写入动作会让响应头被提前提交
		c.Header("Content-Encoding", "gzip")
		origin.WriteHeader(status)
		_, _ = gz.Write(buf.body)
		_ = gz.Close()
		gz.Reset(io.Discard)
		gzipWriterPool.Put(gz)
	}
}

// cacheControlMiddleware 返回 Gin 中间件，设置静态资源的缓存策略。
func cacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		// 哈希文件名的静态资源：缓存 1 年
		if strings.HasPrefix(path, "/assets/") && strings.Contains(path, "-") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		}
		// HTML 文档（SPA 全部前端路由）：不缓存，保证发布后用户拿到最新
		// 仅 /assets/ 与 /api/ 例外（前者已设长缓存，后者是接口与媒体文件）
		if !strings.HasPrefix(path, "/assets/") && !strings.HasPrefix(path, "/api/") {
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		}
		c.Next()
	}
}

// splitStr 按分隔符分割字符串，并去除空白项。
func splitStr(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
