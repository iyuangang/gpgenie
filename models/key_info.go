package models

import "gorm.io/gorm"

type KeyInfo struct {
	gorm.Model
	Fingerprint           string `gorm:"uniqueIndex"`
	PublicKey             string `gorm:"type:text"`
	PrivateKey            string `gorm:"type:text"`
	RepeatLetterScore     int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore      int
	Score                 int
	UniqueLettersCount    int
}
