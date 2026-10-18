package logger

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogger 接收明确的日志级别和格式，不再与具体的 SIT/PROD 环境字面量绑定
// level: "debug", "info", "warn", "error"
// format: "json", "text"
func InitLogger(levelStr string, formatStr string) {
	// 1. 解析日志级别 (默认为 Info)
	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error", "err", "fatal", "critical":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 2. 配置通用的 Handler 配置
	// 无论什么环境，建议都开启 AddSource（显示文件名和行号），方便 AI 和人类定位 Bug
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	}

	// 3. 根据格式（Format）动态生成 Handler
	var handler slog.Handler
	if strings.ToLower(formatStr) == "text" {
		// 本地开发或测试推荐 text 格式，更直观
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		// 生产或容器环境推荐 json 格式，方便 ELK/Loki 采集
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	// 4. 设置为全局默认日志器
	slog.SetDefault(slog.New(handler))
}

// 保持不变的全局便捷包装函数
func Info(msg string, args ...any) { slog.Info(msg, args...) }

func Debug(msg string, args ...any) { slog.Debug(msg, args...) }

func Warn(msg string, args ...any) { slog.Warn(msg, args...) }

func Error(msg string, err error, args ...any) {
	args = append(args, "error", err)
	slog.Error(msg, args...)
}
