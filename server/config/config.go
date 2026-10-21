// config 包负责加载应用程序配置，支持 YAML 文件和环境变量覆盖。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// 存储后端驱动选项，database.driver 配置二选一。
const (
	DriverMySQL  = "mysql"
	DriverSQLite = "sqlite"
)

// Config 存储所有应用程序配置项，按 server / database / storage / scheduler 四组划分。
type Config struct {
	// Server 服务自身配置：监听端口、鉴权密钥、目录与日志等。
	Server ServerConfig `yaml:"server"`
	// Database 数据库配置：driver 二选一（sqlite 默认），sqlite/mysql 各一组参数。
	Database DatabaseConfig `yaml:"database"`
	// Storage 对象存储 + CDN 配置（七牛），含浏览器缓存策略。
	Storage StorageConfig `yaml:"storage"`
	// Scheduler 定时任务配置：每个任务可独立设置启用开关与执行周期。
	Scheduler SchedulerConfig `yaml:"scheduler"`
}

// ServerConfig 服务配置组。
type ServerConfig struct {
	// Port HTTP 监听端口（仅端口号，不带冒号）。
	Port string `yaml:"port"`
	// JWTSecret JWT 签名密钥（必填）：生成/校验登录令牌，更换后所有登录态失效。
	JWTSecret string `yaml:"jwt_secret"`
	// UploadDir 上传文件本地存储目录。
	UploadDir string `yaml:"upload_dir"`
	// LogLevel 日志级别：debug / info / warn / error。
	LogLevel string `yaml:"log_level"`
	// LogDir 日志输出目录。
	LogDir string `yaml:"log_dir"`
	// WebDistDir 前端构建产物目录（服务端托管静态页面）。
	WebDistDir string `yaml:"web_dist_dir"`
	// RevealFreePerDay 每个IP每日可直接查看房东完整电话的次数（自然日重置）。
	// 超过该次数后，每次查看都要求通过图片验证码；0 表示每次查看都要验证码。
	RevealFreePerDay int `yaml:"reveal_free_per_day"`
}

// DatabaseConfig 数据库配置组。
type DatabaseConfig struct {
	// Driver 存储后端选择：sqlite（默认，纯 Go 实现、零安装）或 mysql。
	Driver string `yaml:"driver"`
	// SQLite 仅 driver=sqlite 时生效。
	SQLite SQLiteConfig `yaml:"sqlite"`
	// MySQL 仅 driver=mysql 时生效。
	MySQL MySQLConfig `yaml:"mysql"`
}

// SQLiteConfig SQLite 配置（driver=sqlite 时生效）。
type SQLiteConfig struct {
	// Path 数据库文件路径，所在目录不存在时自动创建。
	Path string `yaml:"path"`
}

// MySQLConfig MySQL 连接参数（driver=mysql 时生效）。
type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Name     string `yaml:"name"`
}

// JobConfig 单个定时任务的调度配置。Time 与 IntervalHours 二选一：
// Time 非空时按「每天 HH:MM」执行；否则按 IntervalHours 每隔 N 小时执行。
type JobConfig struct {
	// Enabled 是否启用该任务的自动调度（手动触发接口不受影响）。
	Enabled bool `yaml:"enabled"`
	// Time 每天定时执行时刻，格式 "HH:MM"。
	Time string `yaml:"time"`
	// IntervalHours 每隔 N 小时执行一次（Time 为空时生效）。
	IntervalHours int `yaml:"interval_hours"`
	// Keep 对象存储中保留的备份份数（仅 db_backup 用），<=0 时取默认值。
	Keep int `yaml:"keep"`
}

// SchedulerConfig 定时任务配置组，每个字段对应一个具名任务。
type SchedulerConfig struct {
	// RentBills 月度租金账单生成：为缺少当月账单的合同生成账单。
	RentBills JobConfig `yaml:"rent_bills"`
	// BuildingExpiry 公寓到期检查：30天内到期建提醒，已到期下架。
	BuildingExpiry JobConfig `yaml:"building_expiry"`
	// ContractExpiry 到期合同检查：30天内到期建待办 + 预订超时转逾期。
	ContractExpiry JobConfig `yaml:"contract_expiry"`
	// SoftDeleteCleanup 软删除清理：清 90 天前软删数据与浏览记录。
	SoftDeleteCleanup JobConfig `yaml:"soft_delete_cleanup"`
	// TokenCleanup 吊销令牌清理：清内存中过期的吊销记录。
	TokenCleanup JobConfig `yaml:"token_cleanup"`
	// DBBackup 数据库全量备份到对象存储（sqlite: VACUUM INTO；mysql: mysqldump）。
	DBBackup JobConfig `yaml:"db_backup"`
}

// StorageConfig 对象存储 + CDN 配置组。
type StorageConfig struct {
	// MediaMaxAge 媒体文件浏览器缓存时长（秒），默认一周。文件 key 均为 uuid 不会
	// 覆盖同名内容，故始终带 immutable；调大可省流量，调小可让客户端更快失效。
	// 注意：七牛对象元数据里也会写入该值（上传时 x-qn-meta-cache-control），
	// 修改配置后存量文件需重跑 scripts/fixcache 才会对直连/CDN 模式生效；
	// 代理模式下服务端响应头始终以本配置为准，无需 fixcache。
	MediaMaxAge int `yaml:"media_max_age"`
	// Qiniu 七牛对象存储 + CDN 参数。
	Qiniu QiniuConfig `yaml:"qiniu"`
}

// QiniuConfig 七牛对象存储/CDN 配置。
type QiniuConfig struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Bucket    string `yaml:"bucket"`
	// Domain CDN 绑定域名（不带协议）。
	Domain string `yaml:"domain"`
	// UseHTTPS 是否以 https 访问 CDN/七牛资源。
	UseHTTPS bool `yaml:"use_https"`
	// Zone 存储区域，如 z0/z1/z2/na0/as0。
	Zone string `yaml:"zone"`
	// ProxyMode 七牛媒体访问模式：
	//   true  (默认)  = 代理模式。浏览器只请求本站 /api/media/xxx，由服务器从
	//                  七牛拉取字节流后透传回去，浏览器永远不与七牛域名直接通信。
	//                  适用：七牛使用 HTTP 非备案测试域名(如 *.clouddn.com)，
	//                  微信内置浏览器会拦截这类资源导致图片/视频无法显示。
	//                  代价：流量经过服务器，占用服务器上行/下行带宽。
	//   false          = 直连模式。服务器对 /api/media/xxx 返回 302 重定向，
	//                  浏览器直接向七牛 CDN 拉取文件，不占用服务器带宽，走 CDN 加速。
	//                  适用：正式备案的 HTTPS CDN 域名(此时建议 use_https=true)。
	//                  代价：若域名仍为非备案 HTTP，微信端资源会被拦截而无法显示。
	ProxyMode bool `yaml:"proxy_mode"`
	// PfopEnable 七牛转码开关：true 时视频上传后自动触发七牛持久化转码(720p)，
	// 转码结果经回调写回，原片删除；false 则视频按原件存储。
	PfopEnable bool `yaml:"pfop_enable"`
	// PfopCallbackSecret 七牛转码回调校验密钥，防止伪造回调。
	PfopCallbackSecret string `yaml:"pfop_callback_secret"`
	// PfopCallbackURL 七牛转码回调地址(服务器公网可访问)；留空则自动用请求 Host 推导。
	PfopCallbackURL string `yaml:"pfop_callback_url"`
}

// defaults 返回默认配置值。
func defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Port:             "8080",
			JWTSecret:        "",
			UploadDir:        "./storage/media",
			LogLevel:         "info",
			LogDir:           "./logs",
			WebDistDir:       "../web/dist",
			RevealFreePerDay: 5,
		},
		Database: DatabaseConfig{
			Driver: DriverSQLite,
			SQLite: SQLiteConfig{
				Path: "./storage/rental.db",
			},
			MySQL: MySQLConfig{
				Host:     "127.0.0.1",
				Port:     "3306",
				User:     "root",
				Password: "",
				Name:     "rental",
			},
		},
		Storage: StorageConfig{
			MediaMaxAge: 604800, // 一周
			Qiniu: QiniuConfig{
				ProxyMode: true,
				Zone:      "z2",
			},
		},
		Scheduler: SchedulerConfig{
			RentBills:         JobConfig{Enabled: true, Time: "03:00"},
			BuildingExpiry:    JobConfig{Enabled: true, Time: "03:00"},
			ContractExpiry:    JobConfig{Enabled: true, Time: "03:00"},
			SoftDeleteCleanup: JobConfig{Enabled: true, Time: "03:00"},
			TokenCleanup:      JobConfig{Enabled: true, IntervalHours: 12},
			DBBackup:          JobConfig{Enabled: true, Time: "04:00", Keep: 7},
		},
	}
}

// MediaCacheControl 返回媒体文件的 Cache-Control 响应头。
func (c *Config) MediaCacheControl() string {
	age := c.Storage.MediaMaxAge
	if age <= 0 {
		age = 604800
	}
	return fmt.Sprintf("public, max-age=%d, immutable", age)
}

// Load 加载配置：先加载 YAML 文件，再用环境变量覆盖，最后校验必填项。
func Load() *Config {
	cfg := defaults()

	if p := os.Getenv("CONFIG_PATH"); p != "" {
		loadFile(p, cfg)
	} else {
		loadFile("config.yaml", cfg)
	}

	envOverrides(cfg)

	normalize(cfg)

	if cfg.Database.Driver == DriverMySQL && cfg.Database.MySQL.Password == "" {
		panic(fmt.Sprintf("database password is required: set database.mysql.password in config.yaml or DB_PASSWORD env"))
	}
	if cfg.Server.JWTSecret == "" {
		panic(fmt.Sprintf("jwt secret is required: set server.jwt_secret in config.yaml or JWT_SECRET env"))
	}

	return cfg
}

// normalize 归一化驱动选项：小写、空值与非法值兜底，补齐 SQLite 文件路径默认值。
func normalize(cfg *Config) {
	cfg.Database.Driver = strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = DriverSQLite
	}
	if cfg.Database.Driver != DriverMySQL && cfg.Database.Driver != DriverSQLite {
		panic(fmt.Sprintf("不支持的 database.driver: %q（可选 mysql / sqlite）", cfg.Database.Driver))
	}
	if cfg.Database.SQLite.Path == "" {
		cfg.Database.SQLite.Path = "./storage/rental.db"
	}
	if cfg.Server.RevealFreePerDay < 0 {
		cfg.Server.RevealFreePerDay = 5
	}
	normalizeJob("rent_bills", &cfg.Scheduler.RentBills, "03:00", 0)
	normalizeJob("building_expiry", &cfg.Scheduler.BuildingExpiry, "03:00", 0)
	normalizeJob("contract_expiry", &cfg.Scheduler.ContractExpiry, "03:00", 0)
	normalizeJob("soft_delete_cleanup", &cfg.Scheduler.SoftDeleteCleanup, "03:00", 0)
	normalizeJob("token_cleanup", &cfg.Scheduler.TokenCleanup, "", 12)
	normalizeJob("db_backup", &cfg.Scheduler.DBBackup, "04:00", 0)
	if cfg.Scheduler.DBBackup.Keep <= 0 {
		cfg.Scheduler.DBBackup.Keep = 7
	}
}

// normalizeJob 校验并兜底单个定时任务的调度配置：time 必须是合法 "HH:MM"，
// 否则回退默认值；daily 任务与 interval 任务都没有有效周期时同样回退。
// time 优先：同时配置了 interval_hours 时以 time 为准并清空间隔。
func normalizeJob(name string, j *JobConfig, defaultTime string, defaultInterval int) {
	if j.Time != "" {
		h, m, err := parseHHMM(j.Time)
		if err != nil {
			panic(fmt.Sprintf("scheduler.%s.time 格式错误: %q（应为 HH:MM）", name, j.Time))
		}
		j.Time = fmt.Sprintf("%02d:%02d", h, m)
		j.IntervalHours = 0
		return
	}
	if j.IntervalHours > 0 {
		return
	}
	if defaultInterval > 0 {
		j.IntervalHours = defaultInterval
		return
	}
	j.Time = defaultTime
}

// parseHHMM 解析 "HH:MM" 为小时与分钟（允许单位数，拒绝多余内容）。
func parseHHMM(s string) (int, int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time: %s", s)
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time: %s", s)
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid time: %s", s)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("time out of range: %s", s)
	}
	return h, m, nil
}

// loadFile 从 YAML 文件读取配置并解析到 Config 结构体。
func loadFile(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		panic(fmt.Sprintf("配置解析失败: %v", err))
	}
}

// envOverrides 用环境变量覆盖配置项。
func envOverrides(cfg *Config) {
	if v := os.Getenv("DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.Database.SQLite.Path = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.MySQL.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.Database.MySQL.Port = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.MySQL.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.MySQL.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.MySQL.Name = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Server.JWTSecret = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.Server.Port = v
	}
	if v := os.Getenv("UPLOAD_DIR"); v != "" {
		cfg.Server.UploadDir = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("LOG_DIR"); v != "" {
		cfg.Server.LogDir = v
	}
	if v := os.Getenv("QINIU_ACCESS_KEY"); v != "" {
		cfg.Storage.Qiniu.AccessKey = v
	}
	if v := os.Getenv("QINIU_SECRET_KEY"); v != "" {
		cfg.Storage.Qiniu.SecretKey = v
	}
	if v := os.Getenv("QINIU_BUCKET"); v != "" {
		cfg.Storage.Qiniu.Bucket = v
	}
	if v := os.Getenv("QINIU_DOMAIN"); v != "" {
		cfg.Storage.Qiniu.Domain = v
	}
	if v := os.Getenv("QINIU_USE_HTTPS"); v != "" {
		cfg.Storage.Qiniu.UseHTTPS = v == "true"
	}
	if v := os.Getenv("QINIU_PROXY_MODE"); v != "" {
		cfg.Storage.Qiniu.ProxyMode = v == "true"
	}
	if v := os.Getenv("QINIU_PFOP_ENABLE"); v != "" {
		cfg.Storage.Qiniu.PfopEnable = v == "true"
	}
	if v := os.Getenv("QINIU_PFOP_CALLBACK_SECRET"); v != "" {
		cfg.Storage.Qiniu.PfopCallbackSecret = v
	}
	if v := os.Getenv("QINIU_PFOP_CALLBACK_URL"); v != "" {
		cfg.Storage.Qiniu.PfopCallbackURL = v
	}
	if v := os.Getenv("QINIU_ZONE"); v != "" {
		cfg.Storage.Qiniu.Zone = v
	}
	if v := os.Getenv("WEB_DIST_DIR"); v != "" {
		cfg.Server.WebDistDir = v
	}
	if v := os.Getenv("MEDIA_MAX_AGE"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Storage.MediaMaxAge = n
		}
	}
}
