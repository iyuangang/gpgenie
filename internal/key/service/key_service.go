package service

import (
	"context"
	"fmt"
	"sync"

	"gpgenie/internal/config"
	"gpgenie/internal/key/domain"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"
	"gpgenie/models"
)

// KeyService 定义了密钥服务的接口
type KeyService interface {
	GenerateKeys(ctx context.Context) error
	ShowTopKeys(n int) error
	ShowLowLetterCountKeys(n int) error
	ExportKeyByFingerprint(lastSixteen, outputDir string, exportArmor bool) error
	AnalyzeData() error
}

// keyService 是 KeyService 接口的具体实现
type keyService struct {
	repo      repository.KeyRepository
	config    config.KeyGenerationConfig
	encryptor domain.Encryptor
	logger    *logger.Logger
}

// NewKeyService 创建一个新的 KeyService 实例，通过依赖注入传入 Encryptor 接口
func NewKeyService(repo repository.KeyRepository, cfg config.KeyGenerationConfig, encryptor domain.Encryptor, log *logger.Logger) KeyService {
	return &keyService{
		repo:      repo,
		config:    cfg,
		encryptor: encryptor,
		logger:    log,
	}
}

// GenerateKeys 实现 GenerateKeys 方法
func (s *keyService) GenerateKeys(ctx context.Context) error {
	cfg := s.config
	workerCount := cfg.NumWorkers
	jobCount := cfg.TotalKeys

	jobs := make(chan struct{}, workerCount*10000)
	results := make(chan *models.KeyInfo, cfg.BatchSize*20)

	var wg sync.WaitGroup

	// 启动 Workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go s.worker(i, ctx, jobs, results, &wg)
		s.logger.Debugf("Worker %d 启动。", i)
	}

	// 加载 Encryptor，仅在 GenerateKeys 时加载
	encryptorPublicKey := s.config.EncryptorPublicKey
	if encryptorPublicKey != "" {
		var err error
		s.encryptor, err = NewPGPEncryptor(encryptorPublicKey)
		if err != nil {
			s.logger.Errorf("加载公钥失败: %v", err)
			return fmt.Errorf("加载公钥失败: %w", err)
		}
	}

	insertWg := sync.WaitGroup{}
	insertWg.Add(1)
	go func() {
		defer insertWg.Done()
		var localBatch []*models.KeyInfo

		// 开始事务
		tx := s.repo.BeginTransaction()
		defer func() {
			if r := recover(); r != nil {
				if err := tx.Rollback(); err != nil {
					s.logger.Errorf("回滚事务失败: %v", err)
				}
			}
		}()

		for key := range results {
			if key != nil { // 确保 key 不为 nil
				localBatch = append(localBatch, key)
				if len(localBatch) >= cfg.BatchSize {
					if err := tx.BatchCreate(localBatch); err != nil {
						s.logger.Errorf("插入批次失败: %v", err)
						if err := tx.Rollback(); err != nil {
							s.logger.Errorf("回滚事务失败: %v", err)
						}
						return
					}
					s.logger.Infof("插入了 %d 个密钥。", len(localBatch))
					localBatch = nil
				}
			}
		}
		// 插入剩余的密钥
		if len(localBatch) > 0 {
			if err := tx.BatchCreate(localBatch); err != nil {
				s.logger.Errorf("插入剩余批次失败: %v", err)
				if err := tx.Rollback(); err != nil {
					s.logger.Errorf("回滚事务失败: %v", err)
				}
				return
			}
			s.logger.Infof("插入了 %d 个密钥。", len(localBatch))
		}

		// 提交事务
		if err := tx.Commit(); err != nil {
			s.logger.Errorf("事务提交失败: %v", err)
			return
		}
	}()

	// 发送 Jobs
	go func() {
		for i := 0; i < jobCount; i++ {
			select {
			case jobs <- struct{}{}:
			case <-ctx.Done():
				close(jobs)
				return
			}
		}
		close(jobs)
	}()

	// 等待 Workers 完成
	wg.Wait()
	close(results)

	// 等待插入 Workers 完成
	insertWg.Wait()

	s.logger.Info("密钥生成过程完成。")
	return nil
}

// worker 是 Worker Pool 的单个 Worker，负责生成和评分密钥
func (s *keyService) worker(id int, ctx context.Context, jobs <-chan struct{}, results chan<- *models.KeyInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	s.logger.Debugf("Worker %d 开始工作。", id)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debugf("Worker %d 接收到取消信号。", id)
			return
		case _, ok := <-jobs:
			if !ok {
				s.logger.Debugf("Worker %d 完成工作。", id)
				return
			}
			keyInfo, err := domain.GenerateAndScoreKeyPair(s.config, s.encryptor)
			if keyInfo != nil && err == nil {
				select {
				case results <- keyInfo:
				case <-ctx.Done():
					return
				}
			} else if err != nil {
				s.logger.Errorf("Worker %d 生成密钥失败: %v", id, err)
			}
		}
	}
}

// ShowTopKeys 实现 ShowTopKeys 方法
func (s *keyService) ShowTopKeys(n int) error {
	keys, err := s.repo.GetTopKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve top keys: %w", err)
	}

	domain.DisplayKeys(keys)
	return nil
}

// ShowLowLetterCountKeys 实现 ShowLowLetterCountKeys 方法
func (s *keyService) ShowLowLetterCountKeys(n int) error {
	keys, err := s.repo.GetLowLetterCountKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve low letter count keys: %w", err)
	}

	domain.DisplayKeys(keys)
	return nil
}

// ExportKeyByFingerprint 实现 ExportKeyByFingerprint 方法
func (s *keyService) ExportKeyByFingerprint(lastSixteen, outputDir string, exportArmor bool) error {
	keyInfo, err := s.repo.GetByFingerprint(lastSixteen)
	if err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	return domain.ExportKey(keyInfo, outputDir, exportArmor, s.encryptor, s.logger)
}

// AnalyzeData 实现 AnalyzeData 方法
func (s *keyService) AnalyzeData() error {
	analyzer := domain.NewAnalyzer(s.repo)
	return analyzer.PerformAnalysis()
}
