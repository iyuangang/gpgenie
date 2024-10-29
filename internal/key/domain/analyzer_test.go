package domain

import (
	"testing"

	"gpgenie/internal/repository"
	"gpgenie/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockKeyRepository 实现完整的 KeyRepository 接口
type MockKeyRepository struct {
	mock.Mock
}

// 实现 KeyRepository 接口的所有方法
func (m *MockKeyRepository) BatchCreate(keys []*models.KeyInfo) error {
	args := m.Called(keys)
	return args.Error(0)
}

func (m *MockKeyRepository) GetTopKeys(limit int) ([]models.KeyInfo, error) {
	args := m.Called(limit)
	return args.Get(0).([]models.KeyInfo), args.Error(1)
}

func (m *MockKeyRepository) GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error) {
	args := m.Called(limit)
	return args.Get(0).([]models.KeyInfo), args.Error(1)
}

func (m *MockKeyRepository) GetByFingerprint(fingerprint string) (*models.KeyInfo, error) {
	args := m.Called(fingerprint)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.KeyInfo), args.Error(1)
}

func (m *MockKeyRepository) GetAll() ([]models.KeyInfo, error) {
	args := m.Called()
	return args.Get(0).([]models.KeyInfo), args.Error(1)
}

func (m *MockKeyRepository) GetScoreStats() (*repository.ScoreStats, error) {
	args := m.Called()
	return args.Get(0).(*repository.ScoreStats), args.Error(1)
}

func (m *MockKeyRepository) GetUniqueLettersStats() (*repository.UniqueLettersStats, error) {
	args := m.Called()
	return args.Get(0).(*repository.UniqueLettersStats), args.Error(1)
}

func (m *MockKeyRepository) GetScoreComponentsStats() (*repository.ScoreComponentsStats, error) {
	args := m.Called()
	return args.Get(0).(*repository.ScoreComponentsStats), args.Error(1)
}

func (m *MockKeyRepository) GetCorrelationCoefficient() (float64, error) {
	args := m.Called()
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockKeyRepository) BeginTransaction() repository.RepositoryTransaction {
	args := m.Called()
	return args.Get(0).(repository.RepositoryTransaction)
}

// MockRepositoryTransaction 实现 RepositoryTransaction 接口
type MockRepositoryTransaction struct {
	mock.Mock
}

func (m *MockRepositoryTransaction) BatchCreate(keys []*models.KeyInfo) error {
	args := m.Called(keys)
	return args.Error(0)
}

func (m *MockRepositoryTransaction) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockRepositoryTransaction) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func TestAnalyzer_PerformAnalysis(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	analyzer := NewAnalyzer(mockRepo)

	// Setup mock expectations
	mockRepo.On("GetScoreStats").Return(&repository.ScoreStats{
		Average: 100.0,
		Min:     50.0,
		Max:     150.0,
		Count:   10,
	}, nil)

	mockRepo.On("GetUniqueLettersStats").Return(&repository.UniqueLettersStats{
		Average: 8.0,
		Min:     5.0,
		Max:     12.0,
		Count:   10,
	}, nil)

	mockRepo.On("GetScoreComponentsStats").Return(&repository.ScoreComponentsStats{
		AverageRepeat:     30.0,
		AverageIncreasing: 40.0,
		AverageDecreasing: 20.0,
		AverageMagic:      10.0,
	}, nil)

	mockRepo.On("GetCorrelationCoefficient").Return(0.75, nil)

	// Execute test
	err := analyzer.PerformAnalysis()

	// Verify results
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

