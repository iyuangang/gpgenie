package repository

import (
	"gpgenie/internal/key/models"

	"gorm.io/gorm"
)

// KeyRepository 定义了与 KeyInfo 相关的数据库操作
type KeyRepository interface {
	BatchCreateKeyInfo(keys []*models.KeyInfo) error
	GetTopKeys(limit int) ([]models.KeyInfo, error)
	GetLowLetterCountKeys(limit int) ([]models.KeyInfo, error)
	GetKeyByFingerprint(lastSixteen string) (*models.KeyInfo, error)
	AutoMigrate() error
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
