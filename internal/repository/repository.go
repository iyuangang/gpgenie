package repository

import (
	"errors"
	"math"
	"strings"

	"github.com/iyuangang/gpgenie/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// KeyRepository 定义了与 KeyInfo 相关的数据库操作
type KeyRepository interface {
	BatchCreate(keys []*models.KeyInfo) error
	Upsert(key *models.KeyInfo) error
	GetTopKeys(limit int) ([]models.KeyInfo, error)
	GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error)
	GetByFingerprint(lastSixteen string) (*models.KeyInfo, error)
	GetAnalysisStats() (*AnalysisStats, error)
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

// AnalysisStats contains all aggregates required by the analyze command. The
// repository populates it with one database scan.
type AnalysisStats struct {
	Score         ScoreStats
	UniqueLetters UniqueLettersStats
	Components    ScoreComponentsStats
	Correlation   float64
}

// keyRepository 是 KeyRepository 的具体实现
type keyRepository struct {
	db *gorm.DB
}

// NewKeyRepository 创建一个新的 KeyRepository 实例
func NewKeyRepository(db *gorm.DB) KeyRepository {
	return &keyRepository{db: db}
}

func (r *keyRepository) Upsert(key *models.KeyInfo) error {
	if key == nil {
		return errors.New("key is nil")
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "fingerprint"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"fingerprint_suffix", "primary_fingerprint", "public_key", "private_key",
			"repeat_letter_score", "increasing_letter_score", "decreasing_letter_score",
			"magic_letter_score", "score", "unique_letters_count", "is_vanity",
			"vanity_run_length", "vanity_run_start", "vanity_digit", "vanity_scope",
			"vanity_target_digits", "updated_at",
		}),
	}).Create(key).Error
}

func (r *keyRepository) BatchCreate(keys []*models.KeyInfo) error {
	if len(keys) == 0 {
		return nil
	}
	return r.db.Create(keys).Error
}

func (r *keyRepository) GetTopKeys(limit int) ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Select("fingerprint", "score", "unique_letters_count").
		Order("score DESC, unique_letters_count ASC").Limit(limit).Find(&keys).Error
	return keys, err
}

func (r *keyRepository) GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error) {
	var keys []models.KeyInfo
	err := r.db.Select("fingerprint", "score", "unique_letters_count").
		Order("unique_letters_count ASC, score DESC").Limit(limit).Find(&keys).Error
	return keys, err
}

func (r *keyRepository) GetByFingerprint(lastSixteen string) (*models.KeyInfo, error) {
	var keyInfo models.KeyInfo
	suffix := strings.ToLower(lastSixteen)
	if len(suffix) > 16 {
		suffix = suffix[len(suffix)-16:]
	}
	err := r.db.Where("fingerprint_suffix = ?", suffix).First(&keyInfo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Compatibility fallback for databases created before fingerprint_suffix
		// was introduced. Connect backfills these rows during normal startup.
		err = r.db.Where("fingerprint LIKE ?", "%"+suffix).First(&keyInfo).Error
	}
	if err != nil {
		return nil, err
	}
	return &keyInfo, nil
}

func (r *keyRepository) GetAnalysisStats() (*AnalysisStats, error) {
	type aggregateRow struct {
		Count             int64   `gorm:"column:count"`
		ScoreAverage      float64 `gorm:"column:score_average"`
		ScoreMin          float64 `gorm:"column:score_min"`
		ScoreMax          float64 `gorm:"column:score_max"`
		ScoreTotal        float64 `gorm:"column:score_total"`
		UniqueAverage     float64 `gorm:"column:unique_average"`
		UniqueMin         float64 `gorm:"column:unique_min"`
		UniqueMax         float64 `gorm:"column:unique_max"`
		UniqueTotal       float64 `gorm:"column:unique_total"`
		AverageRepeat     float64 `gorm:"column:average_repeat"`
		AverageIncreasing float64 `gorm:"column:average_increasing"`
		AverageDecreasing float64 `gorm:"column:average_decreasing"`
		AverageMagic      float64 `gorm:"column:average_magic"`
		SumXY             float64 `gorm:"column:sum_xy"`
		SumX2             float64 `gorm:"column:sum_x2"`
		SumY2             float64 `gorm:"column:sum_y2"`
	}

	var row aggregateRow
	err := r.db.Model(&models.KeyInfo{}).Select(`
		COUNT(*) AS count,
		COALESCE(AVG(score), 0) AS score_average,
		COALESCE(MIN(score), 0) AS score_min,
		COALESCE(MAX(score), 0) AS score_max,
		COALESCE(SUM(score), 0) AS score_total,
		COALESCE(AVG(unique_letters_count), 0) AS unique_average,
		COALESCE(MIN(unique_letters_count), 0) AS unique_min,
		COALESCE(MAX(unique_letters_count), 0) AS unique_max,
		COALESCE(SUM(unique_letters_count), 0) AS unique_total,
		COALESCE(AVG(repeat_letter_score), 0) AS average_repeat,
		COALESCE(AVG(increasing_letter_score), 0) AS average_increasing,
		COALESCE(AVG(decreasing_letter_score), 0) AS average_decreasing,
		COALESCE(AVG(magic_letter_score), 0) AS average_magic,
		COALESCE(SUM(1.0 * score * unique_letters_count), 0) AS sum_xy,
		COALESCE(SUM(1.0 * score * score), 0) AS sum_x2,
		COALESCE(SUM(1.0 * unique_letters_count * unique_letters_count), 0) AS sum_y2
	`).Scan(&row).Error
	if err != nil {
		return nil, err
	}

	correlation := 0.0
	if row.Count > 0 {
		n := float64(row.Count)
		numerator := n*row.SumXY - row.ScoreTotal*row.UniqueTotal
		denominatorSquared := (n*row.SumX2 - row.ScoreTotal*row.ScoreTotal) *
			(n*row.SumY2 - row.UniqueTotal*row.UniqueTotal)
		if denominatorSquared > 0 {
			correlation = numerator / math.Sqrt(denominatorSquared)
		}
	}

	return &AnalysisStats{
		Score: ScoreStats{
			Average: row.ScoreAverage,
			Min:     row.ScoreMin,
			Max:     row.ScoreMax,
			Total:   row.ScoreTotal,
			Count:   row.Count,
		},
		UniqueLetters: UniqueLettersStats{
			Average: row.UniqueAverage,
			Min:     row.UniqueMin,
			Max:     row.UniqueMax,
			Total:   row.UniqueTotal,
			Count:   row.Count,
		},
		Components: ScoreComponentsStats{
			AverageRepeat:     row.AverageRepeat,
			AverageIncreasing: row.AverageIncreasing,
			AverageDecreasing: row.AverageDecreasing,
			AverageMagic:      row.AverageMagic,
		},
		Correlation: correlation,
	}, nil
}
