package key

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gpgenie/internal/config"
	"gpgenie/internal/key/models"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"

	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

type Scorer struct {
	config    *config.Config
	encryptor *Encryptor
	repo      repository.KeyRepository
}

// NewScorer 初始化一个新的 Scorer 实例
func NewScorer(repo repository.KeyRepository, cfg *config.Config) (*Scorer, error) {
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
		config:    cfg,
		encryptor: encryptor,
		repo:      repo,
	}

	if err := s.createTablesIfNotExist(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return s, nil
}

// createTablesIfNotExist 创建必要的数据库表
func (s *Scorer) createTablesIfNotExist() error {
	if err := s.repo.AutoMigrate(); err != nil {
		logger.Logger.Fatalf("Failed to auto-migrate tables: %v", err)
		return err
	}
	return nil
}

// GenerateKeys 使用优化后的 Worker Pool 模式并发生成和评分密钥
func (s *Scorer) GenerateKeys() error {
	cfg := s.config.KeyGeneration
	workerCount := cfg.NumWorkers
	jobCount := cfg.TotalKeys

	// 初始化带有合理缓冲区的通道，以平衡内存使用和任务分派效率
	jobs := make(chan struct{}, workerCount*1000)
	results := make(chan *models.KeyInfo, jobCount)

	var wg sync.WaitGroup

	// 启动 Workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.worker(i, jobs, results, &wg)
		logger.Logger.Infof("Worker %d launched.", i)
	}

	// 分发 Jobs
	go func() {
		for i := 0; i < jobCount; i++ {
			jobs <- struct{}{}
		}
		close(jobs)
	}()

	// 启动多个插入Worker
	insertWorkers := 4 // 根据您的数据库性能和硬件调整
	insertWg := sync.WaitGroup{}
	insertWg.Add(insertWorkers)
	for i := 0; i < insertWorkers; i++ {
		go func(workerID int) {
			defer insertWg.Done()
			localBatch := make([]*models.KeyInfo, 0, s.config.Processing.BatchSize)
			for keyInfo := range results {
				localBatch = append(localBatch, keyInfo)
				if len(localBatch) >= s.config.Processing.BatchSize {
					if err := s.repo.BatchCreateKeyInfo(localBatch); err != nil {
						logger.Logger.Errorf("Insert Worker %d failed to insert batch: %v", workerID, err)
					}
					localBatch = localBatch[:0]
				}
			}
			// 插入任何剩余的batch
			if len(localBatch) > 0 {
				if err := s.repo.BatchCreateKeyInfo(localBatch); err != nil {
					logger.Logger.Errorf("Insert Worker %d failed to insert final batch: %v", workerID, err)
				}
			}
		}(i)
	}

	// 等待所有 Workers 完成
	wg.Wait()
	close(results)

	// 等待所有插入Workers完成
	insertWg.Wait()

	logger.Logger.Info("Key generation process completed.")
	return nil
}

// worker 是优化后的 Worker Pool 的单个 Worker，负责生成和评分密钥
func (s *Scorer) worker(id int, jobs <-chan struct{}, results chan<- *models.KeyInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	logger.Logger.Infof("Worker %d started.", id)
	for range jobs {
		keyInfo, err := s.generateAndScoreKeyPair()
		if keyInfo != nil {
			if err == nil {
				results <- keyInfo
			}
		}
	}
	logger.Logger.Infof("Worker %d finished working.", id)
}

// generateAndScoreKeyPair 生成单个密钥对并计算其分数
func (s *Scorer) generateAndScoreKeyPair() (*models.KeyInfo, error) {
	cfg := s.config.KeyGeneration
	entity, err := NewEntity(&cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	fingerprint := fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)
	scores := CalculateScores(fingerprint[len(fingerprint)-16:])
	totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore + scores.DecreasingLetterScore + scores.MagicLetterScore

	// 修改此处：不符合标准时返回 (nil, nil) 而不是错误
	if totalScore <= cfg.MinScore && scores.UniqueLettersCount >= cfg.MaxLettersCount {
		return nil, nil
	}

	pubKeyStr, privKeyStr, err := SerializeKeys(entity, s.encryptor)
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

// ExportKeyByFingerprint 根据指纹的后16位导出密钥到文件
func (s *Scorer) ExportKeyByFingerprint(lastSixteen string, outputDir string, exportArmor bool) error {
	keyInfo, err := s.repo.GetKeyByFingerprint(lastSixteen)
	if err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	// Ensure the output directory exists
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outputFile := filepath.Join(outputDir, keyInfo.Fingerprint+".gpg")

	if exportArmor {
		// Directly export the ASCII Armor-encoded private key
		if err := os.WriteFile(outputFile, []byte(keyInfo.PrivateKey), 0o600); err != nil {
			return fmt.Errorf("failed to write ASCII Armor private key to file: %w", err)
		}
	} else {
		// Decode ASCII Armor-encoded private key before exporting
		block, err := armor.Decode(strings.NewReader(keyInfo.PrivateKey))
		if err != nil {
			return fmt.Errorf("failed to decode ASCII Armor private key: %w", err)
		}

		var buf bytes.Buffer
		_, err = io.Copy(&buf, block.Body)
		if err != nil {
			return fmt.Errorf("failed to read decoded private key: %w", err)
		}
		decodedPrivateKey := buf.Bytes()

		if err := os.WriteFile(outputFile, decodedPrivateKey, 0o600); err != nil {
			return fmt.Errorf("failed to write decoded private key to file: %w", err)
		}
	}

	logger.Logger.Infof("Successfully exported key to %s", outputFile)
	return nil
}

// ShowTopKeys 在控制台显示评分最高的前 N 个密钥
func (s *Scorer) ShowTopKeys(n int) error {
	keys, err := s.repo.GetTopKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve top keys: %w", err)
	}

	displayKeys(keys)
	return nil
}

// ShowLowLetterCountKeys 在控制台显示字母计数最低的前 N 个密钥
func (s *Scorer) ShowLowLetterCountKeys(n int) error {
	keys, err := s.repo.GetLowLetterCountKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve low letter count keys: %w", err)
	}

	displayKeys(keys)
	return nil
}

// displayKeys 在控制台以格式化表格显示密钥
func displayKeys(keys []models.KeyInfo) {
	fmt.Println("Fingerprint      Score  Letters Count")
	fmt.Println("---------------- ------ -------------")
	for _, key := range keys {
		shortFingerprint := strings.ToUpper(key.Fingerprint[len(key.Fingerprint)-16:])
		fmt.Printf("%-16s %6d %13d\n", shortFingerprint, key.Score, key.UniqueLettersCount)
	}
}
