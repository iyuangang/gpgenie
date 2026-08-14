package repository

import (
	"testing"

	"github.com/iyuangang/gpgenie/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
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
		{Fingerprint: "00000000fingerprint1", FingerprintSuffix: "0000fingerprint1", Score: 100, UniqueLettersCount: 10},
		{Fingerprint: "00000000fingerprint2", FingerprintSuffix: "0000fingerprint2", Score: 200, UniqueLettersCount: 8},
		{Fingerprint: "00000000fingerprint3", FingerprintSuffix: "0000fingerprint3", Score: 150, UniqueLettersCount: 12},
	}

	err := repo.BatchCreate(keys)
	assert.NoError(t, err)

	topKeys, err := repo.GetTopKeys(2)
	assert.NoError(t, err)
	assert.Len(t, topKeys, 2)
	assert.Equal(t, "00000000fingerprint2", topKeys[0].Fingerprint)
	assert.Equal(t, "00000000fingerprint3", topKeys[1].Fingerprint)

	found, err := repo.GetByFingerprint("0000fingerprint2")
	assert.NoError(t, err)
	assert.Equal(t, 200, found.Score)

	stats, err := repo.GetAnalysisStats()
	assert.NoError(t, err)
	assert.Equal(t, int64(3), stats.Score.Count)
	assert.InDelta(t, 150.0, stats.Score.Average, 0.001)
}
