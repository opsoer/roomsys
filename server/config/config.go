// config 包负责加载应用程序配置，支持 JSON 文件和环境变量覆盖。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// 存储后端驱动选项，db_driver 配置二选一。
const (
	DriverMySQL  = "mysql"
	DriverSQLite = "sqlite"
)

// Config 存储所有应用程序配置项。
type Config struct {
	// DBDriver 存储后端选择：mysql（默认，兼容老配置）或 sqlite。
	DBDriver string `json:"db_driver"`
	// DBPath SQLite 数据库文件路径，仅 db_driver=sqlite 时使用。
	DBPath string `json:"db_path"`
	// DBHost 以下为 MySQL 连接参数，仅 db_driver=mysql 时使用。
	DBHost     string `json:"db_host"`
	DBPort     string `json:"db_port"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	DBName     string `json:"db_name"`
	JWTSecret  string `json:"jwt_secret"`
	ServerPort string `json:"server_port"`
	UploadDir  string `json:"upload_dir"`
	LogLevel   string `json:"log_level"`
	LogDir     string `json:"log_dir"`
	WebDistDir string `json:"web_dist_dir"`

	QiniuAccessKey string `json:"qiniu_access_key"`
	QiniuSecretKey string `json:"qiniu_secret_key"`
	QiniuBucket    string `json:"qiniu_bucket"`
	QiniuDomain    string `json:"qiniu_domain"`
	QiniuUseHTTPS  bool   `json:"qiniu_use_https"`
	QiniuZone      string `json:"qiniu_zone"`
	// QiniuProxyMode 七牛媒体访问模式：
	//   true  (默认)  = 代理模式。浏览器只请求本站 /api/media/xxx，由服务器从
	//                  七牛拉取字节流后透传回去，浏览器永远不与七牛域名直接通信。
	//                  适用：七牛使用 HTTP 非备案测试域名(如 *.clouddn.com)，
	//                  微信内置浏览器会拦截这类资源导致图片/视频无法显示。
	//                  代价：流量经过服务器，占用服务器上行/下行带宽。
	//   false          = 直连模式。服务器对 /api/media/xxx 返回 302 重定向，
	//                  浏览器直接向七牛 CDN 拉取文件，不占用服务器带宽，走 CDN 加速。
	//                  适用：正式备案的 HTTPS CDN 域名(此时建议 qiniu_use_https=true)。
	//                  代价：若域名仍为非备案 HTTP，微信端资源会被拦截而无法显示。
	QiniuProxyMode bool `json:"qiniu_proxy_mode"`
	// QiniuPfopEnable 七牛转码开关：true 时视频上传后自动触发七牛持久化转码(720p)，
	// 转码结果经回调写回，原片删除；false 则视频按原件存储。
	QiniuPfopEnable bool `json:"qiniu_pfop_enable"`
	// QiniuPfopCallbackSecret 七牛转码回调校验密钥，防止伪造回调。
	QiniuPfopCallbackSecret string `json:"qiniu_pfop_callback_secret"`
	// QiniuPfopCallbackURL 七牛转码回调地址(服务器公网可访问)；留空则自动用请求 Host 推导。
	QiniuPfopCallbackURL string `json:"qiniu_pfop_callback_url"`
}

// defaults 返回默认配置值。
func defaults() *Config {
	return &Config{
		DBDriver:       DriverMySQL,
		DBPath:         "./storage/rental.db",
		DBHost:         "127.0.0.1",
		DBPort:         "3306",
		DBUser:         "root",
		DBPassword:     "",
		DBName:         "rental",
		JWTSecret:      "",
		ServerPort:     "8080",
		UploadDir:      "./storage/media",
		LogLevel:       "info",
		LogDir:         "./logs",
		WebDistDir:     "../web/dist",
		QiniuProxyMode: true,
	}
}

// Load 加载配置：先加载 JSON 文件，再用环境变量覆盖，最后校验必填项。
func Load() *Config {
	cfg := defaults()

	if p := os.Getenv("CONFIG_PATH"); p != "" {
		loadFile(p, cfg)
	} else {
		loadFile("config.json", cfg)
	}

	envOverrides(cfg)

	normalize(cfg)

	if cfg.DBDriver == DriverMySQL && cfg.DBPassword == "" {
		panic(fmt.Sprintf("database password is required: set db_password in config.json or DB_PASSWORD env"))
	}
	if cfg.JWTSecret == "" {
		panic(fmt.Sprintf("jwt secret is required: set jwt_secret in config.json or JWT_SECRET env"))
	}

	return cfg
}

// normalize 归一化驱动选项：小写、空值与非法值兜底，补齐 SQLite 文件路径默认值。
func normalize(cfg *Config) {
	cfg.DBDriver = strings.ToLower(strings.TrimSpace(cfg.DBDriver))
	if cfg.DBDriver == "" {
		cfg.DBDriver = DriverMySQL
	}
	if cfg.DBDriver != DriverMySQL && cfg.DBDriver != DriverSQLite {
		panic(fmt.Sprintf("不支持的 db_driver: %q（可选 mysql / sqlite）", cfg.DBDriver))
	}
	if cfg.DBPath == "" {
		cfg.DBPath = "./storage/rental.db"
	}
}

// loadFile 从 JSON 文件读取配置并解析到 Config 结构体。
func loadFile(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		panic(fmt.Sprintf("配置解析失败: %v", err))
	}
}

// envOverrides 用环境变量覆盖配置项。
func envOverrides(cfg *Config) {
	if v := os.Getenv("DB_DRIVER"); v != "" {
		cfg.DBDriver = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.DBPort = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		cfg.ServerPort = v
	}
	if v := os.Getenv("UPLOAD_DIR"); v != "" {
		cfg.UploadDir = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("LOG_DIR"); v != "" {
		cfg.LogDir = v
	}
	if v := os.Getenv("QINIU_ACCESS_KEY"); v != "" {
		cfg.QiniuAccessKey = v
	}
	if v := os.Getenv("QINIU_SECRET_KEY"); v != "" {
		cfg.QiniuSecretKey = v
	}
	if v := os.Getenv("QINIU_BUCKET"); v != "" {
		cfg.QiniuBucket = v
	}
	if v := os.Getenv("QINIU_DOMAIN"); v != "" {
		cfg.QiniuDomain = v
	}
	if v := os.Getenv("QINIU_USE_HTTPS"); v != "" {
		cfg.QiniuUseHTTPS = v == "true"
	}
	if v := os.Getenv("QINIU_PROXY_MODE"); v != "" {
		cfg.QiniuProxyMode = v == "true"
	}
	if v := os.Getenv("QINIU_PFOP_ENABLE"); v != "" {
		cfg.QiniuPfopEnable = v == "true"
	}
	if v := os.Getenv("QINIU_PFOP_CALLBACK_SECRET"); v != "" {
		cfg.QiniuPfopCallbackSecret = v
	}
	if v := os.Getenv("QINIU_PFOP_CALLBACK_URL"); v != "" {
		cfg.QiniuPfopCallbackURL = v
	}
	if v := os.Getenv("QINIU_ZONE"); v != "" {
		cfg.QiniuZone = v
	}
	if v := os.Getenv("WEB_DIST_DIR"); v != "" {
		cfg.WebDistDir = v
	}
}
