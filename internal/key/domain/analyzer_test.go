package domain

import (
	"testing"

	"github.com/iyuangang/gpgenie/internal/repository"
	"github.com/iyuangang/gpgenie/models"

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

func (m *MockKeyRepository) Upsert(key *models.KeyInfo) error {
	args := m.Called(key)
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

func (m *MockKeyRepository) GetAnalysisStats() (*repository.AnalysisStats, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.AnalysisStats), args.Error(1)
}

func TestAnalyzer_PerformAnalysis(t *testing.T) {
	mockRepo := new(MockKeyRepository)
	analyzer := NewAnalyzer(mockRepo)

	// Setup mock expectations
	mockRepo.On("GetAnalysisStats").Return(&repository.AnalysisStats{
		Score: repository.ScoreStats{
			Average: 100.0,
			Min:     50.0,
			Max:     150.0,
			Count:   10,
		},
		UniqueLetters: repository.UniqueLettersStats{
			Average: 8.0,
			Min:     5.0,
			Max:     12.0,
			Count:   10,
		},
		Components: repository.ScoreComponentsStats{
			AverageRepeat:     30.0,
			AverageIncreasing: 40.0,
			AverageDecreasing: 20.0,
			AverageMagic:      10.0,
		},
		Correlation: 0.75,
	}, nil)

	// Execute test
	err := analyzer.PerformAnalysis()

	// Verify results
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}
