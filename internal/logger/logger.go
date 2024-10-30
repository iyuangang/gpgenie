package logger

import (
	"bytes"
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
	buffer      *bytes.Buffer
	bufferMu    sync.Mutex
}

// NewAsyncWriteSyncer 创建一个新的 AsyncWriteSyncer
func NewAsyncWriteSyncer(ws zapcore.WriteSyncer, bufferSize int) *AsyncWriteSyncer {
	aws := &AsyncWriteSyncer{
		writeSyncer: ws,
		logChan:     make(chan []byte, bufferSize),
		closeCh:     make(chan struct{}),
		buffer:      bytes.NewBuffer(make([]byte, 0, 4096)), // 预分配缓冲区
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
				// 在关闭前刷新缓冲区
				err := aws.flushBuffer()
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to flush buffer: %v\n", err)
				}
				return
			}
			if err := aws.writeData(p); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
			}
		case <-aws.closeCh:
			// 在关闭前刷新缓冲区
			if err := aws.flushBuffer(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to flush buffer: %v\n", err)
			}
			return
		}
	}
}

// writeData 处理日志数据的写入
func (aws *AsyncWriteSyncer) writeData(p []byte) error {
	aws.bufferMu.Lock()
	defer aws.bufferMu.Unlock()

	// 将数据添加到缓冲区
	aws.buffer.Write(p)

	// 如果遇到换行符，则写入整行
	for {
		line, err := aws.buffer.ReadBytes('\n')
		if err != nil {
			// 如果没有完整的行，将数据放回缓冲区
			aws.buffer.Write(line)
			break
		}

		// 写入完整的行
		if _, err := aws.writeSyncer.Write(line); err != nil {
			return err
		}
	}

	// 如果缓冲区太大，强制刷新
	if aws.buffer.Len() > 4096 {
		return aws.flushBuffer()
	}

	return nil
}

// flushBuffer 刷新缓冲区中的所有数据
func (aws *AsyncWriteSyncer) flushBuffer() error {
	aws.bufferMu.Lock()
	defer aws.bufferMu.Unlock()

	if aws.buffer.Len() > 0 {
		data := aws.buffer.Bytes()
		aws.buffer.Reset()
		_, err := aws.writeSyncer.Write(data)
		return err
	}
	return nil
}

// Write 实现 zapcore.WriteSyncer 接口
func (aws *AsyncWriteSyncer) Write(p []byte) (n int, err error) {
	// 创建数据副本，避免数据竞争
	dataCopy := make([]byte, len(p))
	copy(dataCopy, p)

	select {
	case aws.logChan <- dataCopy:
		return len(p), nil
	case <-aws.closeCh:
		return 0, fmt.Errorf("async writer is closed")
	}
}

// Sync 实现 zapcore.WriteSyncer 接口
func (aws *AsyncWriteSyncer) Sync() error {
	aws.closeOnce.Do(func() {
		close(aws.closeCh)
		close(aws.logChan)
	})
	aws.wg.Wait()
	return aws.writeSyncer.Sync()
}

// Logger 封装了 zap.SugaredLogger
type Logger struct {
	*zap.SugaredLogger
	asyncWriters []*AsyncWriteSyncer
}

// InitLogger 初始化 Logger
func InitLogger(cfg *config.LoggingConfig) (*Logger, error) {
	atomicLevel := zap.NewAtomicLevel()
	if err := atomicLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.LineEnding = zapcore.DefaultLineEnding

	var cores []zapcore.Core
	var asyncWriters []*AsyncWriteSyncer

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
		fileAsyncWS := NewAsyncWriteSyncer(fileWS, 10000)
		asyncWriters = append(asyncWriters, fileAsyncWS)
		cores = append(cores, zapcore.NewCore(fileEncoder, fileAsyncWS, atomicLevel))
	}

	core := zapcore.NewTee(cores...)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	return &Logger{
		SugaredLogger: logger.Sugar(),
		asyncWriters:  asyncWriters,
	}, nil
}

// SyncLogger 同步日志
func (l *Logger) SyncLogger() {
	for _, writer := range l.asyncWriters {
		if err := writer.Sync(); err != nil && !isStdoutSyncError(err) {
			fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
		}
	}
	if err := l.Sync(); err != nil && !isStdoutSyncError(err) {
		fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
}

func isStdoutSyncError(err error) bool {
	return err.Error() == "sync /dev/stdout: The handle is invalid."
}
