package key

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gpgenie/internal/config"
	"gpgenie/internal/key/models"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"
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

	// 初始化带有较小缓冲区的通道，以控制内存使用
	jobs := make(chan struct{}, workerCount*2)
	results := make(chan *models.KeyInfo, workerCount*2)
	errorsChan := make(chan error, workerCount*2)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 Worker 并传递 context
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.worker(ctx, i, jobs, results, errorsChan, &wg)
		logger.Logger.Infof("Worker %d launched.", i)
	}

	// 分发 Jobs
	go func() {
		for i := 0; i < jobCount; i++ {
			select {
			case jobs <- struct{}{}:
			case <-ctx.Done():
				logger.Logger.Warn("Job dispatching stopped due to cancellation.")
				return
			}
		}
		close(jobs)
	}()

	// 等待 Workers 完成并关闭结果和错误通道
	go func() {
		wg.Wait()
		close(results)
		close(errorsChan)
	}()

	// 收集结果并批量插入数据库
	batch := make([]*models.KeyInfo, 0, s.config.Processing.BatchSize)
	insertedCount := 0
	for {
		select {
		case keyInfo, ok := <-results:
			if !ok {
				results = nil
			} else {
				batch = append(batch, keyInfo)
				if len(batch) >= s.config.Processing.BatchSize {
					if err := s.repo.BatchCreateKeyInfo(batch); err != nil {
						logger.Logger.Errorf("Failed to insert key batch: %v", err)
						// 可选：在数据库错误时取消
						// cancel()
					} else {
						insertedCount += len(batch)
						logger.Logger.Infof("Inserted a batch of %d keys. Total inserted: %d", len(batch), insertedCount)
					}
					batch = batch[:0]
				}
			}
		case err, ok := <-errorsChan:
			if !ok {
				errorsChan = nil
			} else if err != nil {
				logger.Logger.Warnf("Error during key generation: %v", err)
				// 根据需要决定是否取消
				// cancel()
			}
		}

		if results == nil && errorsChan == nil {
			break
		}
	}

	// 插入任何剩余的 batch
	if len(batch) > 0 {
		if err := s.repo.BatchCreateKeyInfo(batch); err != nil {
			logger.Logger.Errorf("Failed to insert final key batch: %v", err)
		} else {
			insertedCount += len(batch)
			logger.Logger.Infof("Inserted the final batch of %d keys. Total inserted: %d", len(batch), insertedCount)
		}
	}

	logger.Logger.Info("Key generation process completed.")
	return nil
}

// worker 是优化后的 Worker Pool 的单个 Worker，负责生成和评分密钥
func (s *Scorer) worker(ctx context.Context, id int, jobs <-chan struct{}, results chan<- *models.KeyInfo, errorsChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	logger.Logger.Infof("Worker %d started.", id)
	taskCount := 0
	skippedCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.Logger.Infof("Worker %d received cancellation signal.", id)
			return
		case _, ok := <-jobs:
			if !ok {
				logger.Logger.Infof("Worker %d stopping as jobs channel is closed.", id)
				return
			}
			keyInfo, err := s.generateAndScoreKeyPair()
			if err != nil {
				errorsChan <- fmt.Errorf("worker %d: %w", id, err)
				// 可选：在关键错误时取消 context
				// cancel()
			} else if keyInfo != nil {
				results <- keyInfo
				taskCount++
				if taskCount%10 == 0 { // 每处理10个任务记录一次日志
					logger.Logger.Infof("Worker %d has processed %d tasks.", id, taskCount)
				}
			} else {
				skippedCount++
			}
		}
	}
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
		// logger.Logger.Debugf("Key %s does not meet criteria (Score: %d, UniqueLettersCount: %d). Skipping.", fingerprint, totalScore, scores.UniqueLettersCount)
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
func (s *Scorer) ExportKeyByFingerprint(lastSixteen string, outputDir string) error {
	keyInfo, err := s.repo.GetKeyByFingerprint(lastSixteen)
	if err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	// 解码私钥
	decodedPrivateKey, err := base64.StdEncoding.DecodeString(keyInfo.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 创建输出文件
	outputFile := filepath.Join(outputDir, keyInfo.Fingerprint+".gpg")
	if err := os.WriteFile(outputFile, decodedPrivateKey, 0o600); err != nil {
		return fmt.Errorf("failed to write encrypted private key to file: %w", err)
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
