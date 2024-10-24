package logger

import (
	"fmt"
	"os"
	"path/filepath"

	"gpgenie/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the global SugaredLogger instance
var Logger *zap.SugaredLogger

// InitLogger initializes the global logger based on the provided configuration
func InitLogger(cfg *config.Config) error {
	var zapConfig zap.Config

	// Configure log level based on the configuration
	switch cfg.Logging.LogLevel {
	case "debug":
		zapConfig = zap.NewDevelopmentConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		zapConfig = zap.NewProductionConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		zapConfig = zap.NewProductionConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		zapConfig = zap.NewProductionConfig()
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Logging.LogLevel)
	}

	// Set the log file if specified
	if cfg.Logging.LogFile != "" {
		// Ensure the log directory exists
		logDir := filepath.Dir(cfg.Logging.LogFile)
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		zapConfig.OutputPaths = []string{cfg.Logging.LogFile, "stderr"}
	} else {
		zapConfig.OutputPaths = []string{"stderr"}
	}

	// Build the logger
	zapLogger, err := zapConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %w", err)
	}

	// Convert to SugaredLogger for formatted logging
	Logger = zapLogger.Sugar()
	return nil
}

// SyncLogger flushes any buffered log entries
func SyncLogger() {
	err := Logger.Sync()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}
