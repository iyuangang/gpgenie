package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/iyuangang/gpgenie/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AsyncWriteSyncer 定义了一个异步的 WriteSyncer
type AsyncWriteSyncer struct {
	writeSyncer zapcore.WriteSyncer
	logChan     chan []byte
	wg          sync.WaitGroup
	closeOnce   sync.Once
	stateMu     sync.RWMutex
	closed      bool
}

// NewAsyncWriteSyncer 创建一个新的 AsyncWriteSyncer
func NewAsyncWriteSyncer(ws zapcore.WriteSyncer, bufferSize int) *AsyncWriteSyncer {
	aws := &AsyncWriteSyncer{
		writeSyncer: ws,
		logChan:     make(chan []byte, bufferSize),
	}
	aws.wg.Add(1)
	go aws.run()
	return aws
}

// run 是后台 goroutine，负责写入日志数据
func (aws *AsyncWriteSyncer) run() {
	defer aws.wg.Done()
	for p := range aws.logChan {
		if _, err := aws.writeSyncer.Write(p); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
		}
	}
}

// Write 实现 zapcore.WriteSyncer 接口
func (aws *AsyncWriteSyncer) Write(p []byte) (n int, err error) {
	aws.stateMu.RLock()
	defer aws.stateMu.RUnlock()
	if aws.closed {
		return 0, fmt.Errorf("async writer is closed")
	}

	dataCopy := make([]byte, len(p))
	copy(dataCopy, p)
	aws.logChan <- dataCopy
	return len(p), nil
}

// Sync 实现 zapcore.WriteSyncer 接口
func (aws *AsyncWriteSyncer) Sync() error {
	aws.closeOnce.Do(func() {
		aws.stateMu.Lock()
		aws.closed = true
		close(aws.logChan)
		aws.stateMu.Unlock()
	})
	aws.wg.Wait()
	return aws.writeSyncer.Sync()
}

// Logger 封装了 zap.SugaredLogger
type Logger struct {
	*zap.SugaredLogger
	asyncWriters []*AsyncWriteSyncer
	logFiles     []*os.File
	syncOnce     sync.Once
}

// InitLogger 初始化 Logger
func InitLogger(cfg *config.LoggingConfig) (*Logger, error) {
	atomicLevel := zap.NewAtomicLevel()
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	if err := atomicLevel.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.LineEnding = zapcore.DefaultLineEnding

	var cores []zapcore.Core
	var asyncWriters []*AsyncWriteSyncer
	var logFiles []*os.File

	// 控制台输出
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleWS := zapcore.Lock(os.Stdout)
	consoleAsyncWS := NewAsyncWriteSyncer(consoleWS, 10000)
	asyncWriters = append(asyncWriters, consoleAsyncWS)
	cores = append(cores, zapcore.NewCore(consoleEncoder, consoleAsyncWS, atomicLevel))

	// 文件输出
	if cfg.LogFile != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogFile), os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		logFile, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		fileEncoder := zapcore.NewJSONEncoder(encoderConfig)
		fileWS := zapcore.AddSync(logFile)
		logFiles = append(logFiles, logFile)
		fileAsyncWS := NewAsyncWriteSyncer(fileWS, 10000)
		asyncWriters = append(asyncWriters, fileAsyncWS)
		cores = append(cores, zapcore.NewCore(fileEncoder, fileAsyncWS, atomicLevel))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{
		SugaredLogger: logger.Sugar(),
		asyncWriters:  asyncWriters,
		logFiles:      logFiles,
	}, nil
}

// SyncLogger 同步日志
func (l *Logger) SyncLogger() {
	l.syncOnce.Do(func() {
		for _, writer := range l.asyncWriters {
			if err := writer.Sync(); err != nil && !isStdoutSyncError(err) {
				fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
			}
		}
		if err := l.Sync(); err != nil && !isStdoutSyncError(err) {
			fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
		}
		for _, file := range l.logFiles {
			if err := file.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to close log file: %v\n", err)
			}
		}
	})
}

func isStdoutSyncError(err error) bool {
	var pathErr *os.PathError
	return errors.As(err, &pathErr) && pathErr.Op == "sync" && pathErr.Path == os.Stdout.Name()
}
