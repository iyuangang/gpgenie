package service

import (
	"context"
	"errors"
	"testing"

	"gpgenie/internal/config"
	"gpgenie/internal/key/service/mocks"
	"gpgenie/internal/repository"
	"gpgenie/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository 是仓储的模拟实现
type MockRepository struct {
	mock.Mock
	repository.KeyRepository
}

func (m *MockRepository) AutoMigrate() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRepository) BatchCreate(keys []*models.KeyInfo) error {
	args := m.Called(keys)
	return args.Error(0)
}

func (m *MockRepository) GetTopKeys(limit int) ([]models.KeyInfo, error) {
	args := m.Called(limit)
	keys := args.Get(0)
	if keys == nil {
		return nil, args.Error(1)
	}
	return keys.([]models.KeyInfo), args.Error(1)
}

func (m *MockRepository) GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error) {
	args := m.Called(limit)
	keys := args.Get(0)
	if keys == nil {
		return nil, args.Error(1)
	}
	return keys.([]models.KeyInfo), args.Error(1)
}

func (m *MockRepository) GetByFingerprint(lastSixteen string) (*models.KeyInfo, error) {
	args := m.Called(lastSixteen)
	key := args.Get(0)
	if key == nil {
		return nil, args.Error(1)
	}
	return key.(*models.KeyInfo), args.Error(1)
}

func (m *MockRepository) GetAll() ([]models.KeyInfo, error) {
	args := m.Called()
	keys := args.Get(0)
	if keys == nil {
		return nil, args.Error(1)
	}
	return keys.([]models.KeyInfo), args.Error(1)
}

func (m *MockRepository) GetScoreStats() (*repository.ScoreStats, error) {
	args := m.Called()
	stats := args.Get(0)
	if stats == nil {
		return nil, args.Error(1)
	}
	return stats.(*repository.ScoreStats), args.Error(1)
}

func (m *MockRepository) GetUniqueLettersStats() (*repository.UniqueLettersStats, error) {
	args := m.Called()
	stats := args.Get(0)
	if stats == nil {
		return nil, args.Error(1)
	}
	return stats.(*repository.UniqueLettersStats), args.Error(1)
}

func (m *MockRepository) GetScoreComponentsStats() (*repository.ScoreComponentsStats, error) {
	args := m.Called()
	stats := args.Get(0)
	if stats == nil {
		return nil, args.Error(1)
	}
	return stats.(*repository.ScoreComponentsStats), args.Error(1)
}

func (m *MockRepository) GetCorrelationCoefficient() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func TestGenerateKeys(t *testing.T) {
	// 设置模拟 Encryptor
	mockEncryptor := &mocks.MockEncryptor{
		EncryptFunc: func(plaintext string) (string, error) {
			return "encrypted_" + plaintext, nil
		},
	}

	// 设置模拟仓储
	mockRepo := new(MockRepository)
	mockRepo.On("BatchCreate", mock.Anything).Return(nil)

	// 初始化 Logger（可以使用无操作的 Logger）
	// logger := logger.NewLogger()

	// 配置
	cfg := config.KeyGenerationConfig{
		TotalKeys:       10,
		NumWorkers:      2,
		MinScore:        50,
		MaxLettersCount: 10,
		BatchSize:       5,
		Name:            "Test",
		Comment:         "Testing",
		Email:           "test@example.com",
	}

	// 初始化 KeyService
	keyService := NewKeyService(mockRepo, cfg, mockEncryptor, nil)
	// 执行 GenerateKeys
	err := keyService.GenerateKeys(context.Background())
	assert.NoError(t, err)

	// 验证 BatchCreate 被调用
	mockRepo.AssertCalled(t, "BatchCreate", mock.Anything)
}

func TestGenerateKeys_EncryptError(t *testing.T) {
	// 设置模拟 Encryptor，模拟加密错误
	mockEncryptor := &mocks.MockEncryptor{
		EncryptFunc: func(plaintext string) (string, error) {
			return "", errors.New("encryption failed")
		},
	}

	// 设置模拟仓储
	mockRepo := new(MockRepository)
	// 即使加密失败，可能不会调用 BatchCreate，取决于业务逻辑

	// 初始化 Logger（可以使用无操作的 Logger）
	// logger := logger.NewLogger()

	// 配置
	cfg := config.KeyGenerationConfig{
		TotalKeys:       10,
		NumWorkers:      2,
		MinScore:        50,
		MaxLettersCount: 10,
		BatchSize:       5,
		Name:            "Test",
		Comment:         "Testing",
		Email:           "test@example.com",
	}

	// 初始化 KeyService
	keyService := NewKeyService(mockRepo, cfg, mockEncryptor, nil)

	// 执行 GenerateKeys
	err := keyService.GenerateKeys(context.Background())
	assert.NoError(t, err) // 根据业务逻辑，可能仍然成功但不插入

	// 验证 BatchCreate 未被调用
	mockRepo.AssertNotCalled(t, "BatchCreate", mock.Anything)
}
