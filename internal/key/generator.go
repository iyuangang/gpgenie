package key

import (
	"crypto"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type KeyInfo struct {
	Fingerprint           string
	PublicKey             string
	PrivateKey            string
	RepeatLetterScore     int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore      int
	Score                 int
	UniqueLettersCount    int
}

var keyInfoPool = sync.Pool{
	New: func() interface{} {
		return &KeyInfo{}
	},
}

func (s *Scorer) GenerateKeys() error {
	cfg := s.config.KeyGeneration
	var wg sync.WaitGroup
	keysPerWorker := cfg.TotalKeys / cfg.NumWorkers
	keyInfoChan := make(chan *KeyInfo, cfg.TotalKeys)

	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go func(workerId int) {
			defer wg.Done()
			for j := 0; j < keysPerWorker; j++ {
				if keyInfo, err := s.generateAndScoreKeyPair(); err == nil {
					keyInfoChan <- keyInfo
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(keyInfoChan)
	}()

	return s.processAndInsertKeyInfo(keyInfoChan)
}

func (s *Scorer) generateAndScoreKeyPair() (*KeyInfo, error) {
	cfg := s.config.KeyGeneration
	entity, err := openpgp.NewEntity(cfg.Name, cfg.Comment, cfg.Email, &packet.Config{
		DefaultHash:   crypto.SHA256,
		Time:          time.Now,
		Algorithm:     packet.PubKeyAlgoEdDSA,
		KeyLifetimeSecs: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	fingerprint := fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)
	scores := calculateScores(fingerprint[len(fingerprint)-16:])
	totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore + scores.DecreasingLetterScore + scores.MagicLetterScore
	
	if totalScore <= cfg.MinScore && scores.UniqueLettersCount >= cfg.MaxLettersCount {
		return nil, fmt.Errorf("key does not meet criteria")
	}

	pubKeyBuf := new(strings.Builder)
	privKeyBuf := new(strings.Builder)

	pubKeyArmor, err := armor.Encode(pubKeyBuf, openpgp.PublicKeyType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}
	entity.Serialize(pubKeyArmor)
	pubKeyArmor.Close()

	privKeyArmor, err := armor.Encode(privKeyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key: %w", err)
	}
	entity.SerializePrivate(privKeyArmor, nil)
	privKeyArmor.Close()

	// Encrypt the private key if encryptor is available
	if s.encryptor != nil {
		encryptedPrivateKey, err := s.encryptor.EncryptAndEncode(privKeyBuf.String())
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt private key: %w", err)
		}
		privKeyBuf = new(strings.Builder)
		privKeyBuf.WriteString(encryptedPrivateKey)
	}

	keyInfo := keyInfoPool.Get().(*KeyInfo)
	*keyInfo = KeyInfo{
		Fingerprint:  fingerprint,
		PublicKey:             pubKeyBuf.String(),
		PrivateKey:            privKeyBuf.String(),
		RepeatLetterScore:     scores.RepeatLetterScore,
		IncreasingLetterScore: scores.IncreasingLetterScore,
		DecreasingLetterScore: scores.DecreasingLetterScore,
		MagicLetterScore:      scores.MagicLetterScore,
		Score:                 totalScore,
		UniqueLettersCount:    scores.UniqueLettersCount,
	}
	return keyInfo, nil
}

func (s *Scorer) processAndInsertKeyInfo(keyInfoChan <-chan *KeyInfo) error {
	batch := make([]*KeyInfo, 0, s.config.Processing.BatchSize)
	for keyInfo := range keyInfoChan {
		batch = append(batch, keyInfo)
		if len(batch) >= s.config.Processing.BatchSize {
			if err := s.insertKeyBatch(batch); err != nil {
				return err
			}
			for _, ki := range batch {
				keyInfoPool.Put(ki)
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		return s.insertKeyBatch(batch)
	}
	return nil
}

func (s *Scorer) insertKeyBatch(batch []*KeyInfo) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO gpg_ed25519_keys (fingerprint, public_key, private_key,repeat_letter_score, increasing_letter_score, decreasing_letter_score, magic_letter_score, score, unique_letters_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, keyInfo := range batch {
		_, err := stmt.Exec(
			keyInfo.Fingerprint,
			keyInfo.PublicKey,
			keyInfo.PrivateKey,
			keyInfo.RepeatLetterScore,
			keyInfo.IncreasingLetterScore,
			keyInfo.DecreasingLetterScore,
			keyInfo.MagicLetterScore,
			keyInfo.Score,
			keyInfo.UniqueLettersCount,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
