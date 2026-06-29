package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
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

	QiniuAccessKey string `json:"qiniu_access_key"`
	QiniuSecretKey string `json:"qiniu_secret_key"`
	QiniuBucket    string `json:"qiniu_bucket"`
	QiniuDomain    string `json:"qiniu_domain"`
	QiniuUseHTTPS  bool   `json:"qiniu_use_https"`
	QiniuZone      string `json:"qiniu_zone"`
}

func defaults() *Config {
	return &Config{
		DBHost:     "127.0.0.1",
		DBPort:     "3306",
		DBUser:     "root",
		DBPassword: "",
		DBName:     "rental",
		JWTSecret:  "",
		ServerPort: "8080",
		UploadDir:  "./storage/media",
		LogLevel:   "info",
		LogDir:     "./logs",
	}
}

func Load() *Config {
	cfg := defaults()

	if p := os.Getenv("CONFIG_PATH"); p != "" {
		loadFile(p, cfg)
	} else {
		loadFile("config.json", cfg)
	}

	envOverrides(cfg)

	if cfg.DBPassword == "" {
		panic(fmt.Sprintf("database password is required: set db_password in config.json or DB_PASSWORD env"))
	}
	if cfg.JWTSecret == "" {
		panic(fmt.Sprintf("jwt secret is required: set jwt_secret in config.json or JWT_SECRET env"))
	}

	return cfg
}

func loadFile(path string, cfg *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	json.Unmarshal(data, cfg)
}

func envOverrides(cfg *Config) {
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
	if v := os.Getenv("QINIU_ZONE"); v != "" {
		cfg.QiniuZone = v
	}
}
