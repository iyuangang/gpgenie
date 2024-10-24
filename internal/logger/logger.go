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

// nopSync is a WriteSyncer that ignores the Sync operation.
type nopSync struct {
	zapcore.WriteSyncer
}

// Sync is a no-op for nopSync.
func (n nopSync) Sync() error {
	return nil
}

// InitLogger initializes the global logger based on the provided configuration
func InitLogger(cfg *config.Config) error {
	// Set the log level.
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Logging.LogLevel)); err != nil {
		return fmt.Errorf("invalid log level: %s", cfg.Logging.LogLevel)
	}

	// Create the encoder configuration.
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	// Create console encoder (human-readable).
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create file encoder (JSON).
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	// Prepare cores.
	var cores []zapcore.Core

	// Console output core with no-op Sync.
	consoleWriteSyncer := nopSync{zapcore.Lock(os.Stdout)}
	consoleCore := zapcore.NewCore(
		consoleEncoder,
		consoleWriteSyncer,
		level,
	)
	cores = append(cores, consoleCore)

	// File output core (if specified).
	if cfg.Logging.LogFile != "" {
		// Ensure the log directory exists.
		logDir := filepath.Dir(cfg.Logging.LogFile)
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}

		// Open the log file.
		logFile, err := os.OpenFile(cfg.Logging.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}

		// File output core with proper Sync.
		fileCore := zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(logFile),
			level,
		)
		cores = append(cores, fileCore)
	}

	// Combine multiple cores.
	multiCore := zapcore.NewTee(cores...)

	// Build the logger.
	zapLogger := zap.New(multiCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	// Convert to SugaredLogger for formatted logging.
	Logger = zapLogger.Sugar()

	// Add application-specific fields.
	Logger = Logger.With("app", "gpgenie", "version", "0.1.0")

	return nil
}

// SyncLogger flushes any buffered log entries
func SyncLogger() {
	if err := Logger.Sync(); err != nil {
		// Check if the error is related to syncing stdout and ignore it.
		if err.Error() != "sync /dev/stdout: The handle is invalid." {
			fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
		}
	}
}
