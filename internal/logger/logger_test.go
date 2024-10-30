package logger

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gpgenie/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWriteSyncer 用于测试的 WriteSyncer
type MockWriteSyncer struct {
	mu      sync.Mutex
	written [][]byte
	synced  bool
	err     error
}

func NewMockWriteSyncer() *MockWriteSyncer {
	return &MockWriteSyncer{
		written: make([][]byte, 0),
	}
}

func (m *MockWriteSyncer) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return 0, m.err
	}
	copied := make([]byte, len(p))
	copy(copied, p)
	m.written = append(m.written, copied)
	return len(p), nil
}

func (m *MockWriteSyncer) Sync() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.synced = true
	return m.err
}

func (m *MockWriteSyncer) GetWritten() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.written))
	for i, b := range m.written {
		result[i] = string(b)
	}
	return result
}

// TestAsyncWriteSyncer_Write tests the Write method
func TestAsyncWriteSyncer_Write(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{
			name:    "single line",
			input:   []string{"test log\n"},
			wantErr: false,
		},
		{
			name:    "multiple lines",
			input:   []string{"line1\n", "line2\n", "line3\n"},
			wantErr: false,
		},
		{
			name:    "partial lines",
			input:   []string{"part1", "part2\n", "part3"},
			wantErr: false,
		},
		{
			name:    "large input",
			input:   []string{strings.Repeat("a", 8192) + "\n"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockWriteSyncer()
			aws := NewAsyncWriteSyncer(mock, 1000)

			// Write test data
			for _, line := range tt.input {
				n, err := aws.Write([]byte(line))
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, len(line), n)
				}
			}

			// Wait for processing
			time.Sleep(100 * time.Millisecond)

			// Sync and check results
			err := aws.Sync()
			assert.NoError(t, err)

			written := mock.GetWritten()
			assert.NotEmpty(t, written)
		})
	}
}

// TestAsyncWriteSyncer_Sync tests the Sync method
func TestAsyncWriteSyncer_Sync(t *testing.T) {
	mock := NewMockWriteSyncer()
	aws := NewAsyncWriteSyncer(mock, 1000)

	// Write some data
	_, err := aws.Write([]byte("test\n"))
	require.NoError(t, err)

	// Test multiple Sync calls
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := aws.Sync()
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	assert.True(t, mock.synced)
}

// TestLogger_InitLogger tests logger initialization
func TestLogger_InitLogger(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	tests := []struct {
		name    string
		cfg     *config.LoggingConfig
		wantErr bool
	}{
		{
			name: "valid config with file",
			cfg: &config.LoggingConfig{
				LogLevel: "debug",
				LogFile:  logFile,
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			cfg: &config.LoggingConfig{
				LogLevel: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := InitLogger(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
				logger.SyncLogger()
			}
		})
	}
}

// TestLogger_Logging tests actual logging
func TestLogger_Logging(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	cfg := &config.LoggingConfig{
		LogLevel: "debug",
		LogFile:  logFile,
	}

	logger, err := InitLogger(cfg)
	require.NoError(t, err)

	// Test different log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logger.SyncLogger()

	// Verify log file contents
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "debug message")
	assert.Contains(t, string(content), "info message")
	assert.Contains(t, string(content), "warn message")
	assert.Contains(t, string(content), "error message")
}

// Benchmark tests
func BenchmarkAsyncWriteSyncer_Write(b *testing.B) {
	mock := NewMockWriteSyncer()
	aws := NewAsyncWriteSyncer(mock, 1000)

	data := []byte("test log message\n")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := aws.Write(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	if err := aws.Sync(); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkLogger_Logging(b *testing.B) {
	tempDir := b.TempDir()
	logFile := filepath.Join(tempDir, "bench.log")

	cfg := &config.LoggingConfig{
		LogLevel: "info",
		LogFile:  logFile,
	}

	logger, err := InitLogger(cfg)
	require.NoError(b, err)

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			logger.Info("benchmark log message")
		}
	})

	logger.SyncLogger()
}

// TestBufferOverflow tests handling of buffer overflow
func TestBufferOverflow(t *testing.T) {
	mock := NewMockWriteSyncer()
	aws := NewAsyncWriteSyncer(mock, 10)

	// Write more data than buffer can hold
	data := bytes.Repeat([]byte("a"), 8192)
	for i := 0; i < 20; i++ {
		_, err := aws.Write(data)
		if err != nil {
			t.Fatal(err)
		}
	}

	if err := aws.Sync(); err != nil {
		t.Fatal(err)
	}

	assert.True(t, mock.synced)
}

// TestConcurrentWrites tests concurrent writing
func TestConcurrentWrites(t *testing.T) {
	mock := NewMockWriteSyncer()
	aws := NewAsyncWriteSyncer(mock, 1000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg := fmt.Sprintf("message %d\n", i)
			_, err := aws.Write([]byte(msg))
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()
	if err := aws.Sync(); err != nil {
		t.Fatal(err)
	}

	written := mock.GetWritten()
	assert.NotEmpty(t, written)
}
