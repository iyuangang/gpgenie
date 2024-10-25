package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"gpgenie/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.SugaredLogger
}

func InitLogger(cfg config.LoggingConfig) (*Logger, error) {
	// 设置日志级别
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}

	// 编码器配置
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// 控制台编码器
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// 文件编码器
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// 准备 cores
	var cores []zapcore.Core

	// 控制台 core
	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.Lock(os.Stdout), level)
	cores = append(cores, consoleCore)

	// 文件 core
	if cfg.LogFile != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.LogFile)
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// 打开日志文件
		logFile, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		fileCore := zapcore.NewCore(fileEncoder, zapcore.AddSync(logFile), level)
		cores = append(cores, fileCore)
	}

	// 合并 cores
	multiCore := zapcore.NewTee(cores...)

	// 构建 logger
	zapLogger := zap.New(multiCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	// 转换为 SugaredLogger
	sugar := zapLogger.Sugar().With("app", "gpgenie", "version", "0.1.0")

	return &Logger{SugaredLogger: sugar}, nil
}

func (l *Logger) SyncLogger() {
	if err := l.Sync(); err != nil && !isStdoutSyncError(err) {
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}

func isStdoutSyncError(err error) bool {
	return err.Error() == "sync /dev/stdout: The handle is invalid."
}
