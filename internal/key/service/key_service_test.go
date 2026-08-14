package service

import (
	"context"
	"errors"
	"testing"

	"github.com/iyuangang/gpgenie/internal/config"
	"github.com/iyuangang/gpgenie/internal/logger"
	"github.com/iyuangang/gpgenie/internal/repository"
	"github.com/iyuangang/gpgenie/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBatchCreate = errors.New("batch create failed")

type testEncryptor struct{}

func (testEncryptor) Encrypt(string) (string, error) { return "encrypted", nil }

type testRepository struct {
	batchErr error
}

func (r *testRepository) BatchCreate([]*models.KeyInfo) error { return r.batchErr }
func (r *testRepository) Upsert(*models.KeyInfo) error        { return r.batchErr }
func (r *testRepository) GetTopKeys(int) ([]models.KeyInfo, error) {
	return nil, nil
}
func (r *testRepository) GetLowLetterCountKeys(int) ([]models.KeyInfo, error) {
	return nil, nil
}
func (r *testRepository) GetByFingerprint(string) (*models.KeyInfo, error) {
	return nil, nil
}
func (r *testRepository) GetAnalysisStats() (*repository.AnalysisStats, error) {
	return &repository.AnalysisStats{}, nil
}

func TestGenerateKeysReturnsPersistenceError(t *testing.T) {
	log, err := logger.InitLogger(&config.LoggingConfig{LogLevel: "warn"})
	require.NoError(t, err)
	t.Cleanup(log.SyncLogger)

	cfg := validKeyGenerationConfig()
	cfg.TotalKeys = 1
	cfg.BatchSize = 1
	service := NewKeyService(&testRepository{batchErr: errBatchCreate}, &cfg, testEncryptor{}, log)

	err = service.GenerateKeys(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errBatchCreate)
}

func TestGenerateKeysHonorsCanceledContext(t *testing.T) {
	log, err := logger.InitLogger(&config.LoggingConfig{LogLevel: "warn"})
	require.NoError(t, err)
	t.Cleanup(log.SyncLogger)

	cfg := validKeyGenerationConfig()
	cfg.TotalKeys = 1_000_000
	service := NewKeyService(&testRepository{}, &cfg, testEncryptor{}, log)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = service.GenerateKeys(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestGenerateKeysValidatesConfiguration(t *testing.T) {
	log, err := logger.InitLogger(&config.LoggingConfig{LogLevel: "warn"})
	require.NoError(t, err)
	t.Cleanup(log.SyncLogger)

	cfg := validKeyGenerationConfig()
	cfg.BatchSize = 0
	service := NewKeyService(&testRepository{}, &cfg, testEncryptor{}, log)

	err = service.GenerateKeys(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch_size")
}

func validKeyGenerationConfig() config.KeyGenerationConfig {
	return config.KeyGenerationConfig{
		NumGeneratorWorkers: 1,
		NumScorerWorkers:    1,
		TotalKeys:           1,
		MinScore:            -1000,
		MaxLettersCount:     16,
		BatchSize:           10,
		Name:                "Test",
		Email:               "test@example.com",
	}
}
