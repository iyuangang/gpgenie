package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/iyuangang/gpgenie/internal/app"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	publicKeyPath := filepath.Join(tempDir, "encryptor_public_key.asc")
	writeTestPublicKey(t, publicKeyPath)

	configPath := filepath.Join(tempDir, "config.json")
	configData, err := json.Marshal(map[string]any{
		"environment": "test",
		"database": map[string]any{
			"type":              "sqlite",
			"dbname":            filepath.Join(tempDir, "gpgenie.db"),
			"max_open_conns":    1,
			"max_idle_conns":    1,
			"conn_max_lifetime": 300,
			"log_level":         "warn",
		},
		"key_generation": map[string]any{
			"num_generator_workers": 2,
			"num_scorer_workers":    2,
			"total_keys":            5,
			"min_score":             -1000,
			"max_letters_count":     16,
			"batch_size":            2,
			"name":                  "Integration Test",
			"comment":               "Generated during tests",
			"email":                 "integration@example.com",
			"encryptor_public_key":  publicKeyPath,
		},
		"logging": map[string]any{
			"log_level": "warn",
			"log_file":  filepath.Join(tempDir, "gpgenie.log"),
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configData, 0o600))

	application, err := app.NewApp(configPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, application.Close()) })

	err = application.KeyService.GenerateKeys(context.Background())
	require.NoError(t, err)

	keys, err := application.Repository.GetTopKeys(10)
	require.NoError(t, err)
	assert.Len(t, keys, 5)

	require.NoError(t, application.KeyService.ShowTopKeys(5))
	require.NoError(t, application.KeyService.AnalyzeData())
}

func writeTestPublicKey(t *testing.T, path string) {
	t.Helper()
	entity, err := openpgp.NewEntity("Test Encryptor", "", "encryptor@example.com", nil)
	require.NoError(t, err)

	var output bytes.Buffer
	armorWriter, err := armor.Encode(&output, openpgp.PublicKeyType, nil)
	require.NoError(t, err)
	require.NoError(t, entity.Serialize(armorWriter))
	require.NoError(t, armorWriter.Close())
	require.NoError(t, os.WriteFile(path, output.Bytes(), 0o600))
}
