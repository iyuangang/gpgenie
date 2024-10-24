package models

import "gorm.io/gorm"

type KeyInfo struct {
	gorm.Model
	Fingerprint           string `gorm:"primaryKey;column:fingerprint"`
	PublicKey             string `gorm:"column:public_key"`
	PrivateKey            string `gorm:"column:private_key"`
	RepeatLetterScore     int    `gorm:"column:repeat_letter_score"`
	IncreasingLetterScore int    `gorm:"column:increasing_letter_score"`
	DecreasingLetterScore int    `gorm:"column:decreasing_letter_score"`
	MagicLetterScore      int    `gorm:"column:magic_letter_score"`
	Score                 int    `gorm:"index;column:score"`
	UniqueLettersCount    int    `gorm:"index;column:unique_letters_count"`
}
