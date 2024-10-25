package repository

import (
	"testing"

	"gpgenie/models"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.KeyInfo{})
	assert.NoError(t, err)
	return db
}

func TestBatchCreateAndGetTopKeys(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)

	keys := []*models.KeyInfo{
		{Fingerprint: "fingerprint1", Score: 100, UniqueLettersCount: 10},
		{Fingerprint: "fingerprint2", Score: 200, UniqueLettersCount: 8},
		{Fingerprint: "fingerprint3", Score: 150, UniqueLettersCount: 12},
	}

	err := repo.BatchCreate(keys)
	assert.NoError(t, err)

	topKeys, err := repo.GetTopKeys(2)
	assert.NoError(t, err)
	assert.Len(t, topKeys, 2)
	assert.Equal(t, "fingerprint2", topKeys[0].Fingerprint)
	assert.Equal(t, "fingerprint3", topKeys[1].Fingerprint)
}
