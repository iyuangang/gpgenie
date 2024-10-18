package key

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gpgenie/internal/config"
	"gpgenie/internal/logger"

	"gorm.io/gorm"
)

type Scorer struct {
	db        *gorm.DB
	config    *config.Config
	encryptor *Encryptor
}

// NewScorer initializes a new Scorer instance
func NewScorer(db *gorm.DB, cfg *config.Config) (*Scorer, error) {
	var encryptor *Encryptor
	var err error
	if cfg.KeyEncryption.PublicKeyPath != "" {
		encryptor, err = NewEncryptor(&cfg.KeyEncryption)
		if err != nil {
			return nil, fmt.Errorf("failed to load encryption public key: %w", err)
		}
		logger.Logger.Info("Encryption public key loaded successfully")
	}

	s := &Scorer{
		db:        db,
		config:    cfg,
		encryptor: encryptor,
	}

	if err := s.createTablesIfNotExist(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return s, nil
}

// createTablesIfNotExist creates the necessary database tables
func (s *Scorer) createTablesIfNotExist() error {
	if err := s.db.AutoMigrate(&KeyInfo{}); err != nil {
		logger.Logger.Fatalf("Failed to auto-migrate gpg_ed25519_keys table: %v", err)
		return err
	}
	return nil
}

// GenerateKeys handles the generation and scoring of keys concurrently
func (s *Scorer) GenerateKeys() error {
	cfg := s.config.KeyGeneration
	var wg sync.WaitGroup
	keysPerWorker := cfg.TotalKeys / cfg.NumWorkers
	keyInfoChan := make(chan *KeyInfo, cfg.TotalKeys)
	errorChan := make(chan error, s.config.KeyGeneration.NumWorkers)

	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < keysPerWorker; j++ {
				keyInfo, err := s.generateAndScoreKeyPair()
				if err == nil {
					keyInfoChan <- keyInfo
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(keyInfoChan)
		close(errorChan)
	}()

	// Insert keys into the database
	if err := s.processAndInsertKeyInfo(keyInfoChan); err != nil {
		return err
	}

	// Check for any errors during key generation
	for err := range errorChan {
		if err != nil {
			logger.Logger.Warnf("Encountered error during key generation: %v", err)
		}
	}

	logger.Logger.Info("Key generation process completed.")
	return nil
}

// generateAndScoreKeyPair generates a single key pair and calculates its scores
func (s *Scorer) generateAndScoreKeyPair() (*KeyInfo, error) {
	cfg := s.config.KeyGeneration
	entity, err := NewEntity(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	fingerprint := fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)
	scores := CalculateScores(fingerprint[len(fingerprint)-16:])
	totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore + scores.DecreasingLetterScore + scores.MagicLetterScore

	if totalScore <= cfg.MinScore && scores.UniqueLettersCount >= cfg.MaxLettersCount {
		return nil, errors.New("key does not meet criteria")
	}

	pubKeyStr, privKeyStr, err := SerializeKeys(entity, s.encryptor)
	if err != nil {
		return nil, err
	}

	keyInfo := &KeyInfo{
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

// processAndInsertKeyInfo processes the channel of KeyInfo and inserts them into the database
func (s *Scorer) processAndInsertKeyInfo(keyInfoChan <-chan *KeyInfo) error {
	batch := make([]*KeyInfo, 0, s.config.Processing.BatchSize)
	for keyInfo := range keyInfoChan {
		batch = append(batch, keyInfo)
		if len(batch) >= s.config.Processing.BatchSize {
			if err := s.insertKeyBatch(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		return s.insertKeyBatch(batch)
	}
	return nil
}

// insertKeyBatch inserts a batch of KeyInfo into the database
func (s *Scorer) insertKeyBatch(batch []*KeyInfo) error {
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&batch).Error
	}); err != nil {
		return fmt.Errorf("failed to insert key batch: %w", err)
	}
	logger.Logger.Infof("Inserted batch of %d keys into the database.", len(batch))
	return nil
}

// ExportTopKeys exports the top N keys by score to a CSV file
func (s *Scorer) ExportTopKeys(limit int) error {
	var keys []KeyInfo
	if err := s.db.Order("score DESC, unique_letters_count").
		Limit(limit).
		Find(&keys).Error; err != nil {
		return fmt.Errorf("failed to retrieve top keys: %w", err)
	}

	outputFile := "top_keys.csv"
	if err := exportKeysToCSV(keys, outputFile); err != nil {
		return err
	}

	logger.Logger.Infof("Exported %d top keys to %s", limit, outputFile)
	return nil
}

// ExportLowLetterCountKeys exports the top N keys with the lowest letter count to a CSV file
func (s *Scorer) ExportLowLetterCountKeys(limit int) error {
	var keys []KeyInfo
	if err := s.db.Where("unique_letters_count < 4").
		Order("unique_letters_count, score DESC").
		Limit(limit).
		Find(&keys).Error; err != nil {
		return fmt.Errorf("failed to retrieve low letter count keys: %w", err)
	}

	outputFile := "low_letter_count_keys.csv"
	if err := exportKeysToCSV(keys, outputFile); err != nil {
		return err
	}

	logger.Logger.Infof("Exported %d low letter count keys to %s", limit, outputFile)
	return nil
}

// ExportKeyByFingerprint exports a key by its last sixteen fingerprint characters to a file
func (s *Scorer) ExportKeyByFingerprint(lastSixteen string, outputDir string) error {
	var key KeyInfo
	fingerprintPattern := "%" + strings.ToLower(lastSixteen)
	if err := s.db.Where("fingerprint LIKE ?", fingerprintPattern).
		First(&key).Error; err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	// Decode the private key
	decodedPrivateKey, err := base64.StdEncoding.DecodeString(key.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	// Ensure the output directory exists
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create the output file
	outputFile := filepath.Join(outputDir, key.Fingerprint+".gpg")
	if err := os.WriteFile(outputFile, decodedPrivateKey, 0o600); err != nil {
		return fmt.Errorf("failed to write encrypted private key to file: %w", err)
	}

	logger.Logger.Infof("Successfully exported key to %s", outputFile)
	return nil
}

// ShowTopKeys displays the top N keys by score in the console
func (s *Scorer) ShowTopKeys(n int) error {
	var keys []ShowKeyInfo
	if err := s.db.Model(&KeyInfo{}).
		Where("score > ?", 400).
		Order("score DESC, unique_letters_count").
		Limit(n).
		Find(&keys).Error; err != nil {
		return fmt.Errorf("failed to retrieve top keys: %w", err)
	}

	displayKeys(keys)
	return nil
}

// ShowLowLetterCountKeys displays the top N keys with the lowest letter count in the console
func (s *Scorer) ShowLowLetterCountKeys(n int) error {
	var keys []ShowKeyInfo
	if err := s.db.Model(&KeyInfo{}).
		Where("unique_letters_count < ?", 4).
		Order("unique_letters_count, score DESC").
		Limit(n).
		Find(&keys).Error; err != nil {
		return fmt.Errorf("failed to retrieve low letter count keys: %w", err)
	}

	displayKeys(keys)
	return nil
}

// displayKeys prints the keys in a formatted table
func displayKeys(keys []ShowKeyInfo) {
	fmt.Println("Fingerprint      Score  Letters Count")
	fmt.Println("---------------- ------ -------------")
	for _, key := range keys {
		shortFingerprint := strings.ToUpper(key.Fingerprint[len(key.Fingerprint)-16:])
		fmt.Printf("%-16s %6d %13d\n", shortFingerprint, key.Score, key.UniqueLettersCount)
	}
}

// exportKeysToCSV exports a list of keys to a CSV file
func exportKeysToCSV(keys []KeyInfo, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write CSV header
	if _, err := file.WriteString("Fingerprint,Score,LettersCount,PublicKey,PrivateKey\n"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write each key as a CSV row
	for _, key := range keys {
		// Escape commas and newlines in the keys
		publicKey := strings.ReplaceAll(key.PublicKey, "\n", "\\n")
		privateKey := strings.ReplaceAll(key.PrivateKey, "\n", "\\n")

		row := fmt.Sprintf("%s,%d,%d,\"%s\",\"%s\"\n",
			strings.ToUpper(key.Fingerprint[len(key.Fingerprint)-16:]),
			key.Score,
			key.UniqueLettersCount,
			publicKey,
			privateKey,
		)
		if _, err := file.WriteString(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return nil
}
