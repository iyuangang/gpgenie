package logger

import (
	"time"

	"gpgenie/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.SugaredLogger

func InitLogger(cfg *config.Config) {
	var zapConfig zap.Config
	if cfg.Database.LogLevel == "info" {
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // 彩色输出
		zapConfig.EncoderConfig.TimeKey = "time"
		zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
	} else {
		zapConfig = zap.NewProductionConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zap.InfoLevel) // 生产环境设置信息级别
		zapConfig.EncoderConfig.TimeKey = "time"
		zapConfig.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	}

	zapLogger, err := zapConfig.Build()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	Logger = zapLogger.Sugar()
}

func SyncLogger() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
