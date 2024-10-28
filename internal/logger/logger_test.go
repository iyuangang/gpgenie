package logger

import (
	"testing"

	"gpgenie/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestInitLogger(t *testing.T) {
	cfg := config.LoggingConfig{
		LogLevel: "debug",
		LogFile:  "logs/gpgenie.log",
	}

	log, err := InitLogger(&cfg)
	assert.NoError(t, err)
	assert.NotNil(t, log)

	log.Debug("This is a debug message.")
	log.Info("This is an info message.")
	log.Warn("This is a warning.")
	log.Error("This is an error.")

	log.SyncLogger()
}
