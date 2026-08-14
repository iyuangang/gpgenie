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
		"vanity": {
			"min_run": 13,
			"save_to_database": true
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
	t.Setenv("GPGENIE_DATABASE_HOST", "database.internal")
	cfg, err := Load(tmpfile.Name())
	require.NoError(t, err)

	// Verify config values
	assert.Equal(t, "test", cfg.Environment)
	assert.Equal(t, "sqlite", cfg.Database.Type)
	assert.Equal(t, "database.internal", cfg.Database.Host)
	assert.Equal(t, 2, cfg.KeyGeneration.NumGeneratorWorkers)
	assert.Equal(t, 13, cfg.Vanity.MinRun)
	assert.True(t, cfg.Vanity.SaveToDatabase)
	assert.Equal(t, "info", cfg.Logging.LogLevel)
}

func TestVanityConfigValidate(t *testing.T) {
	for _, minRun := range []int{0, 1, 13, 16} {
		require.NoError(t, (VanityConfig{MinRun: minRun}).Validate())
	}
	for _, minRun := range []int{-1, 17} {
		assert.Error(t, (VanityConfig{MinRun: minRun}).Validate())
	}
}

func TestKeyGenerationConfigValidate(t *testing.T) {
	valid := KeyGenerationConfig{
		NumGeneratorWorkers: 1,
		NumScorerWorkers:    1,
		TotalKeys:           1,
		BatchSize:           1,
		MaxLettersCount:     16,
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name   string
		mutate func(*KeyGenerationConfig)
	}{
		{"generator workers", func(c *KeyGenerationConfig) { c.NumGeneratorWorkers = 0 }},
		{"scorer workers", func(c *KeyGenerationConfig) { c.NumScorerWorkers = 0 }},
		{"total keys", func(c *KeyGenerationConfig) { c.TotalKeys = -1 }},
		{"batch size", func(c *KeyGenerationConfig) { c.BatchSize = 0 }},
		{"letter count", func(c *KeyGenerationConfig) { c.MaxLettersCount = 17 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			tt.mutate(&cfg)
			assert.Error(t, cfg.Validate())
		})
	}
}
