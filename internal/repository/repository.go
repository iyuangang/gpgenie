package repository

import (
	"gpgenie/internal/key/models"
	"math"

	"gorm.io/gorm"
)

// KeyRepository 定义了与 KeyInfo 相关的数据库操作
type KeyRepository interface {
	BatchCreateKeyInfo(keys []*models.KeyInfo) error
	GetTopKeys(limit int) ([]models.KeyInfo, error)
	GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error)
	GetKeyByFingerprint(lastSixteen string) (*models.KeyInfo, error)
	AutoMigrate() error
	GetAllKeys() ([]models.KeyInfo, error) // 获取所有 KeyInfo
	GetScoreStatistics() (*ScoreStats, error)
	GetUniqueLettersStatistics() (*UniqueLettersStats, error)
	GetScoreComponentsStatistics() (*ScoreComponentsStats, error)
	GetCorrelationCoefficient() (float64, error)
}

// ScoreStats 用于存储分数的统计数据
type ScoreStats struct {
	Average float64
	Min     float64
	Max     float64
	Total   int64
	Count   int64
}

// UniqueLettersStats 用于存储唯一字母计数的统计数据
type UniqueLettersStats struct {
	Average float64
	Min     float64
	Max     float64
	Total   int64
	Count   int64
}

// ScoreComponentsStats 用于存储分数组件的统计数据
type ScoreComponentsStats struct {
	AverageRepeat      float64
	AverageIncreasing  float64
	AverageDecreasing  float64
	AverageMagic       float64
}

// keyRepository 是 KeyRepository 的具体实现
type keyRepository struct {
	db *gorm.DB
}

// NewKeyRepository 创建一个新的 KeyRepository 实例
func NewKeyRepository(db *gorm.DB) KeyRepository {
	return &keyRepository{db: db}
}

// BatchCreateKeyInfo 批量插入 KeyInfo
func (r *keyRepository) BatchCreateKeyInfo(keys []*models.KeyInfo) error {
	return r.db.Create(keys).Error
}

// GetTopKeys 获取评分最高的前 N 个 Key
func (r *keyRepository) GetTopKeys(limit int) ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Order("score DESC").Limit(limit).Find(&keys).Error
	return keys, err
}

// GetLowLetterCountKeys 获取字母计数最低的前 N 个 Key
func (r *keyRepository) GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Order("unique_letters_count ASC").Limit(limit).Find(&keys).Error
	return keys, err
}

// GetKeyByFingerprint 通过指纹的后16位获取 KeyInfo
func (r *keyRepository) GetKeyByFingerprint(lastSixteen string) (*models.KeyInfo, error) {
	var keyInfo models.KeyInfo
	err := r.db.Where("fingerprint LIKE ?", "%"+lastSixteen).First(&keyInfo).Error
	if err != nil {
		return nil, err
	}
	return &keyInfo, nil
}

// AutoMigrate 自动迁移数据库表
func (r *keyRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&models.KeyInfo{}, &models.ShowKeyInfo{})
}

// GetAllKeys 获取数据库中所有的 KeyInfo
func (r *keyRepository) GetAllKeys() ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Find(&keys).Error
	return keys, err
}

// GetScoreStatistics 获取 Score 的统计数据
func (r *keyRepository) GetScoreStatistics() (*ScoreStats, error) {
	var stats ScoreStats
	err := r.db.Model(&models.KeyInfo{}).
		Select("AVG(score) as average, MIN(score) as min, MAX(score) as max, SUM(score) as total, COUNT(score) as count").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetUniqueLettersStatistics 获取 UniqueLettersCount 的统计数据
func (r *keyRepository) GetUniqueLettersStatistics() (*UniqueLettersStats, error) {
	var stats UniqueLettersStats
	err := r.db.Model(&models.KeyInfo{}).
		Select("AVG(unique_letters_count) as average, MIN(unique_letters_count) as min, MAX(unique_letters_count) as max, SUM(unique_letters_count) as total, COUNT(unique_letters_count) as count").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetScoreComponentsStatistics 获取分数组件的统计数据
func (r *keyRepository) GetScoreComponentsStatistics() (*ScoreComponentsStats, error) {
	var stats ScoreComponentsStats
	err := r.db.Model(&models.KeyInfo{}).
		Select("AVG(repeat_letter_score) as average_repeat, AVG(increasing_letter_score) as average_increasing, AVG(decreasing_letter_score) as average_decreasing, AVG(magic_letter_score) as average_magic").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

// GetCorrelationCoefficient 计算 Score 与 UniqueLettersCount 之间的 Pearson 相关系数
func (r *keyRepository) GetCorrelationCoefficient() (float64, error) {
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	var count int64

	rows, err := r.db.Model(&models.KeyInfo{}).Select("score, unique_letters_count").Rows()
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var score int
		var uniqueLettersCount int
		if err := rows.Scan(&score, &uniqueLettersCount); err != nil {
			return 0, err
		}
		x := float64(score)
		y := float64(uniqueLettersCount)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
		count++
	}

	if count == 0 {
		return 0, nil
	}

	numerator := (float64(count)*sumXY - sumX*sumY)
	denominator := math.Sqrt((float64(count)*sumX2 - sumX*sumX) * (float64(count)*sumY2 - sumY*sumY))
	if denominator == 0 {
		return 0, nil
	}

	correlation := numerator / denominator
	return correlation, nil
}
