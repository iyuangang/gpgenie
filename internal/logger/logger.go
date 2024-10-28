package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gpgenie/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AsyncWriteSyncer 定义了一个异步的 WriteSyncer
type AsyncWriteSyncer struct {
	writeSyncer zapcore.WriteSyncer
	logChan     chan []byte
	wg          sync.WaitGroup
	closeOnce   sync.Once
	closeCh     chan struct{}
}

// NewAsyncWriteSyncer 创建一个新的 AsyncWriteSyncer
func NewAsyncWriteSyncer(ws zapcore.WriteSyncer, bufferSize int) *AsyncWriteSyncer {
	aws := &AsyncWriteSyncer{
		writeSyncer: ws,
		logChan:     make(chan []byte, bufferSize),
		closeCh:     make(chan struct{}),
	}
	aws.wg.Add(1)
	go aws.run()
	return aws
}

// run 是后台 goroutine，负责写入日志数据
func (aws *AsyncWriteSyncer) run() {
	defer aws.wg.Done()
	for {
		select {
		case p, ok := <-aws.logChan:
			if !ok {
				return
			}
			if _, err := aws.writeSyncer.Write(p); err != nil {
				// 处理写入错误，例如记录到标准错误输出
				println("Failed to write log:", err.Error())
			}
		case <-aws.closeCh:
			return
		}
	}
}

// Write 实现 zapcore.WriteSyncer 接口，将日志数据发送到 logChan
func (aws *AsyncWriteSyncer) Write(p []byte) (n int, err error) {
	select {
	case aws.logChan <- p:
		return len(p), nil
	default:
		// 当缓冲区满时，阻塞写入
		aws.logChan <- p
		return len(p), nil
	}
}

// Sync 实现 zapcore.WriteSyncer 接口，确保所有日志数据被写入
func (aws *AsyncWriteSyncer) Sync() error {
	aws.closeOnce.Do(func() {
		close(aws.logChan)
		close(aws.closeCh)
	})
	aws.wg.Wait()
	return aws.writeSyncer.Sync()
}

// Close 关闭 AsyncWriteSyncer，确保所有日志数据被写入
func (aws *AsyncWriteSyncer) Close() error {
	aws.closeOnce.Do(func() {
		close(aws.logChan)
		close(aws.closeCh)
	})
	aws.wg.Wait()
	return aws.writeSyncer.Sync()
}

// Logger 封装了 zap.SugaredLogger
type Logger struct {
	*zap.SugaredLogger
}

// InitLogger 初始化 Logger
func InitLogger(cfg *config.LoggingConfig) (*Logger, error) {
	// 设置日志级别
	atomicLevel := zap.NewAtomicLevel()
	if err := atomicLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}

	// 配置编码器
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	fileEncoder := zapcore.NewJSONEncoder(encoderConfig)

	var cores []zapcore.Core

	// 创建控制台 WriteSyncer
	consoleWS := zapcore.Lock(os.Stdout)
	consoleAsyncWS := NewAsyncWriteSyncer(consoleWS, 10000) // 缓冲区大小为10000
	consoleCore := zapcore.NewCore(consoleEncoder, consoleAsyncWS, atomicLevel)
	cores = append(cores, consoleCore)

	// 创建文件 WriteSyncer
	if cfg.LogFile != "" {
		logDir := filepath.Dir(cfg.LogFile)
		if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		logFile, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		fileWS := zapcore.AddSync(logFile)
		fileAsyncWS := NewAsyncWriteSyncer(fileWS, 10000) // 缓冲区大小为10000
		fileCore := zapcore.NewCore(fileEncoder, fileAsyncWS, atomicLevel)
		cores = append(cores, fileCore)
	}

	// 组合多个 Core
	multiCore := zapcore.NewTee(cores...)

	// 创建 Zap Logger
	zapLogger := zap.New(multiCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	sugar := zapLogger.Sugar().With("app", "gpgenie", "version", "0.1.0")

	return &Logger{SugaredLogger: sugar}, nil
}

// SyncLogger 同步日志，确保所有日志被写入
func (l *Logger) SyncLogger() {
	if err := l.Sync(); err != nil && !isStdoutSyncError(err) {
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}

func isStdoutSyncError(err error) bool {
	return err.Error() == "sync /dev/stdout: The handle is invalid."
}
