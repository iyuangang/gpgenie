package logger

import (
	"go.uber.org/zap"
)

var Logger *zap.SugaredLogger

func InitLogger() {
	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	zapLogger, err := cfg.Build()
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	defer zapLogger.Sync()
	Logger = zapLogger.Sugar()
}
