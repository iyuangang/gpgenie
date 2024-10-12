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
	Fingerprint   string
	PublicKey     string
	PrivateKey    string
	RLScore       int
	ILScore       int
	DLScore       int
	MLScore       int
	Score         int
	LettersCount  int
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
				if keyInfo, err := s.generateKeyPair(); err == nil {
					keyInfoChan <- keyInfo
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(keyInfoChan)
	}()

	return s.processKeyInfo(keyInfoChan)
}

func (s *Scorer) generateKeyPair() (*KeyInfo, error) {
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
	totalScore := scores.RLScore + scores.ILScore + scores.DLScore + scores.MLScore
	
	if totalScore <= cfg.MinScore && scores.LettersCount >= cfg.MaxLettersCount {
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

	keyInfo := keyInfoPool.Get().(*KeyInfo)
	*keyInfo = KeyInfo{
		Fingerprint:  fingerprint,
		PublicKey:    pubKeyBuf.String(),
		PrivateKey:   privKeyBuf.String(),
		RLScore:      scores.RLScore,
		ILScore:      scores.ILScore,
		DLScore:      scores.DLScore,
		MLScore:      scores.MLScore,
		Score:        totalScore,
		LettersCount: scores.LettersCount,
	}
	return keyInfo, nil
}

func (s *Scorer) processKeyInfo(keyInfoChan <-chan *KeyInfo) error {
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
		INSERT INTO gpg_ed25519_keys (fingerprint, public_key, private_key,rl_score, il_score, dl_score, ml_score, score, letters_count)
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
			keyInfo.RLScore,
			keyInfo.ILScore,
			keyInfo.DLScore,
			keyInfo.MLScore,
			keyInfo.Score,
			keyInfo.LettersCount,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
