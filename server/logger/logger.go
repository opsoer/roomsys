// logger 包提供基于 zerolog 的日志初始化工具，支持控制台和文件输出。
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// Log 是全局日志实例，通过 Init 初始化后使用。
var Log zerolog.Logger

// Config 定义日志配置：日志级别和输出目录。
type Config struct {
	Level string `json:"log_level"`
	Dir   string `json:"log_dir"`
}

// Init 初始化全局日志实例，同时写入控制台和文件。
func Init(cfg Config) {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(level)

	logDir := cfg.Dir
	if logDir == "" {
		logDir = "./logs"
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败: %v\n", err)
		logDir = "./logs"
		os.MkdirAll(logDir, 0755)
	}

	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	fileWriter, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开日志文件失败，仅使用控制台输出: %v\n", err)
	}

	var writers []io.Writer
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "2006-01-02 15:04:05",
		NoColor:    os.Getenv("NO_COLOR") != "",
	}
	writers = append(writers, consoleWriter)
	if fileWriter != nil {
		writers = append(writers, fileWriter)
	}

	multi := io.MultiWriter(writers...)
	Log = zerolog.New(multi).
		Level(level).
		With().
		Timestamp().
		Caller().
		Logger()
}
