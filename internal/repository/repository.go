package repository

import (
	"math"
	"strings"

	"gpgenie/models"

	"gorm.io/gorm"
)

// KeyRepository 定义了与 KeyInfo 相关的数据库操作
type KeyRepository interface {
	AutoMigrate() error
	BatchCreate(keys []*models.KeyInfo) error
	GetTopKeys(limit int) ([]models.KeyInfo, error)
	GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error)
	GetByFingerprint(lastSixteen string) (*models.KeyInfo, error)
	GetAll() ([]models.KeyInfo, error)
	// 统计方法
	GetScoreStats() (*ScoreStats, error)
	GetUniqueLettersStats() (*UniqueLettersStats, error)
	GetScoreComponentsStats() (*ScoreComponentsStats, error)
	GetCorrelationCoefficient() (float64, error)
}

// ScoreStats 用于存储分数统计数据
type ScoreStats struct {
	Average float64 `gorm:"column:average"`
	Min     float64 `gorm:"column:min"`
	Max     float64 `gorm:"column:max"`
	Total   float64 `gorm:"column:total"`
	Count   int64   `gorm:"column:count"`
}

// UniqueLettersStats 用于存储唯一字母统计数据
type UniqueLettersStats struct {
	Average float64 `gorm:"column:average"`
	Min     float64 `gorm:"column:min"`
	Max     float64 `gorm:"column:max"`
	Total   float64 `gorm:"column:total"`
	Count   int64   `gorm:"column:count"`
}

// ScoreComponentsStats 用于存储分数组件统计数据
type ScoreComponentsStats struct {
	AverageRepeat     float64 `gorm:"column:average_repeat"`
	AverageIncreasing float64 `gorm:"column:average_increasing"`
	AverageDecreasing float64 `gorm:"column:average_decreasing"`
	AverageMagic      float64 `gorm:"column:average_magic"`
}

// keyRepository 是 KeyRepository 的具体实现
type keyRepository struct {
	db *gorm.DB
}

// NewKeyRepository 创建一个新的 KeyRepository 实例
func NewKeyRepository(db *gorm.DB) KeyRepository {
	return &keyRepository{db: db}
}

func (r *keyRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&models.KeyInfo{})
}

func (r *keyRepository) BatchCreate(keys []*models.KeyInfo) error {
	return r.db.Create(keys).Error
}

func (r *keyRepository) GetTopKeys(limit int) ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Order("score DESC, unique_letters_count ASC").Limit(limit).Find(&keys).Error
	return keys, err
}

func (r *keyRepository) GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Order("unique_letters_count ASC, score DESC").Limit(limit).Find(&keys).Error
	return keys, err
}

func (r *keyRepository) GetByFingerprint(lastSixteen string) (*models.KeyInfo, error) {
	var keyInfo models.KeyInfo
	err := r.db.Where("fingerprint LIKE ?", "%"+strings.ToLower(lastSixteen)).First(&keyInfo).Error
	if err != nil {
		return nil, err
	}
	return &keyInfo, nil
}

func (r *keyRepository) GetAll() ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Find(&keys).Error
	return keys, err
}

func (r *keyRepository) GetScoreStats() (*ScoreStats, error) {
	var stats ScoreStats
	err := r.db.Model(&models.KeyInfo{}).
		Select("AVG(score) as average, MIN(score) as min, MAX(score) as max, SUM(score) as total, COUNT(score) as count").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *keyRepository) GetUniqueLettersStats() (*UniqueLettersStats, error) {
	var stats UniqueLettersStats
	err := r.db.Model(&models.KeyInfo{}).
		Select("AVG(unique_letters_count) as average, MIN(unique_letters_count) as min, MAX(unique_letters_count) as max, SUM(unique_letters_count) as total, COUNT(unique_letters_count) as count").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *keyRepository) GetScoreComponentsStats() (*ScoreComponentsStats, error) {
	var stats ScoreComponentsStats
	err := r.db.Model(&models.KeyInfo{}).
		Select("AVG(repeat_letter_score) as average_repeat, AVG(increasing_letter_score) as average_increasing, AVG(decreasing_letter_score) as average_decreasing, AVG(magic_letter_score) as average_magic").
		Scan(&stats).Error
	if err != nil {
		return nil, err
	}
	return &stats, nil
}

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
