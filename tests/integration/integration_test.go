package integration

import (
	"context"
	"testing"

	"gpgenie/internal/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := "../../config/config_test.json"
	app, err := app.NewApp(cfg)
	require.NoError(t, err)
	defer app.Close()

	// Test key generation
	err = app.KeyService.GenerateKeys(context.Background())
	assert.NoError(t, err)

	// Test showing top keys
	err = app.KeyService.ShowTopKeys(5)
	assert.NoError(t, err)

	// Test analysis
	err = app.KeyService.AnalyzeData()
	assert.NoError(t, err)
}
