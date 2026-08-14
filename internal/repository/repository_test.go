package repository

import (
	"testing"

	"github.com/iyuangang/gpgenie/models"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&models.KeyInfo{})
	assert.NoError(t, err)
	return db
}

func TestUpsertVanityKeyIsIdempotent(t *testing.T) {
	db := setupTestDB(t)
	repo := NewKeyRepository(db)
	record := &models.KeyInfo{
		Fingerprint:        "0123456789abcdef01234567abc1111111111111",
		FingerprintSuffix:  "abc1111111111111",
		PrimaryFingerprint: "fedcba9876543210fedcba9876543210fedcba98",
		PublicKey:          "public-v1",
		PrivateKey:         "encrypted-private-v1",
		IsVanity:           true,
		VanityRunLength:    12,
		VanityDigit:        "1",
		VanityScope:        "suffix",
		VanityTargetDigits: "018",
	}
	require.NoError(t, repo.Upsert(record))

	record.PublicKey = "public-v2"
	record.VanityRunLength = 13
	require.NoError(t, repo.Upsert(record))

	got, err := repo.GetByFingerprint(record.FingerprintSuffix)
	require.NoError(t, err)
	assert.Equal(t, "public-v2", got.PublicKey)
	assert.Equal(t, 13, got.VanityRunLength)
	assert.True(t, got.IsVanity)

	var count int64
	require.NoError(t, db.Model(&models.KeyInfo{}).
		Where("fingerprint = ?", record.Fingerprint).
		Count(&count).Error)
	assert.Equal(t, int64(1), count)
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
