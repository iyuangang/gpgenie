package models

import "gorm.io/gorm"

type KeyInfo struct {
	gorm.Model
	Fingerprint           string `gorm:"uniqueIndex"`
	FingerprintSuffix     string `gorm:"size:16;index"`
	PublicKey             string `gorm:"type:text"`
	PrivateKey            string `gorm:"type:text"`
	RepeatLetterScore     int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore      int
	Score                 int `gorm:"index:idx_score_unique,priority:1,sort:desc;index:idx_unique_score,priority:2,sort:desc"`
	UniqueLettersCount    int `gorm:"index:idx_score_unique,priority:2,sort:asc;index:idx_unique_score,priority:1,sort:asc"`
}
