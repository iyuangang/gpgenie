package domain

import (
	"fmt"

	"gpgenie/internal/config"
	"gpgenie/models"
)

// GenerateAndScoreKeyPair 生成一个密钥对并计算其分数
func GenerateAndScoreKeyPair(cfg config.KeyGenerationConfig, encryptor Encryptor) (*models.KeyInfo, error) {
	entity, err := NewEntity(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	fingerprint := fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)
	lastSixteen := fingerprint[len(fingerprint)-16:]
	scores, err := CalculateScores(lastSixteen)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate scores: %w", err)
	}
	totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore + scores.DecreasingLetterScore + scores.MagicLetterScore

	// 不符合标准时返回 (nil, nil)
	if totalScore <= cfg.MinScore || scores.UniqueLettersCount < cfg.MaxLettersCount {
		return nil, nil
	}

	pubKeyStr, privKeyStr, err := SerializeKeys(entity, encryptor)
	if err != nil {
		return nil, err
	}

	keyInfo := &models.KeyInfo{
		Fingerprint:           fingerprint,
		PublicKey:             pubKeyStr,
		PrivateKey:            privKeyStr,
		RepeatLetterScore:     scores.RepeatLetterScore,
		IncreasingLetterScore: scores.IncreasingLetterScore,
		DecreasingLetterScore: scores.DecreasingLetterScore,
		MagicLetterScore:      scores.MagicLetterScore,
		Score:                 totalScore,
		UniqueLettersCount:    scores.UniqueLettersCount,
	}
	return keyInfo, nil
}
