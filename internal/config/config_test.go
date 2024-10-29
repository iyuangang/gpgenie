package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Create temporary config file
	content := `{
		"environment": "test",
		"database": {
			"type": "sqlite",
			"dbname": ":memory:",
			"max_open_conns": 10,
			"max_idle_conns": 5,
			"conn_max_lifetime": 300,
			"log_level": "warn"
		},
		"key_generation": {
			"num_generator_workers": 2,
			"num_scorer_workers": 2,
			"total_keys": 100,
			"min_score": 100,
			"max_letters_count": 8,
			"batch_size": 10,
			"name": "Test Key",
			"email": "test@example.com"
		},
		"logging": {
			"log_level": "info",
			"log_file": "test.log"
		}
	}`

	tmpfile, err := os.CreateTemp("", "config.*.json")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write([]byte(content))
	require.NoError(t, err)
	tmpfile.Close()

	// Test loading config
	cfg, err := Load(tmpfile.Name())
	require.NoError(t, err)

	// Verify config values
	assert.Equal(t, "test", cfg.Environment)
	assert.Equal(t, "sqlite", cfg.Database.Type)
	assert.Equal(t, 2, cfg.KeyGeneration.NumGeneratorWorkers)
	assert.Equal(t, "info", cfg.Logging.LogLevel)
}
