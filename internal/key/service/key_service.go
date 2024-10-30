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

	"github.com/ProtonMail/go-crypto/openpgp"
)

// KeyService 定义了密钥服务的接口
type KeyService interface {
	GenerateKeys(ctx context.Context) error
	ShowTopKeys(n int) error
	ShowMinimalKeys(n int) error
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
	generatorWorkerCount := cfg.NumGeneratorWorkers
	scorerWorkerCount := cfg.NumScorerWorkers
	jobCount := cfg.TotalKeys

	// Channels for pipeline
	generationJobs := make(chan struct{}, generatorWorkerCount*1000)
	generatedEntities := make(chan *openpgp.Entity, jobCount)
	scoredKeyInfos := make(chan *models.KeyInfo, scorerWorkerCount*10000)

	var wgGenerators sync.WaitGroup
	var wgScorers sync.WaitGroup

	// Start Generator Workers
	for i := 0; i < generatorWorkerCount; i++ {
		wgGenerators.Add(1)
		go s.generatorWorker(i, ctx, generationJobs, generatedEntities, &wgGenerators)
		s.logger.Debugf("Generator Worker %d 启动。", i)
	}

	// Start Scorer Workers
	for i := 0; i < scorerWorkerCount; i++ {
		wgScorers.Add(1)
		go s.scorerWorker(i, ctx, generatedEntities, scoredKeyInfos, &wgScorers)
		s.logger.Debugf("Scorer Worker %d 启动。", i)
	}

	// Start Saver Goroutine
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

		for key := range scoredKeyInfos {
			if key != nil {
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

	// Start Enqueue Generation Jobs
	go func() {
		defer close(generationJobs)
		for i := 0; i < jobCount; i++ {
			select {
			case generationJobs <- struct{}{}:
			case <-ctx.Done():
				s.logger.Warn("Enqueue Generation Jobs 被取消。")
				return
			}
		}
	}()

	// Wait for Generators to Finish
	wgGenerators.Wait()
	close(generatedEntities)

	// Wait for Scorers to Finish
	wgScorers.Wait()
	close(scoredKeyInfos)

	// Wait for Saver to Finish
	insertWg.Wait()

	s.logger.Info("密钥生成过程完成。")
	return nil
}

// generatorWorker 是生成密钥的 Worker
func (s *keyService) generatorWorker(id int, ctx context.Context, jobs <-chan struct{}, output chan<- *openpgp.Entity, wg *sync.WaitGroup) {
	defer wg.Done()
	s.logger.Debugf("Generator Worker %d 开始工作。", id)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debugf("Generator Worker %d 接收到取消信号。", id)
			return
		case _, ok := <-jobs:
			if !ok {
				s.logger.Debugf("Generator Worker %d 完成工作。", id)
				return
			}
			entity, err := domain.GenerateKeyPair(s.config, s.encryptor)
			if err != nil {
				s.logger.Errorf("Generator Worker %d 生成密钥失败: %v", id, err)
				continue
			}
			select {
			case output <- entity:
			case <-ctx.Done():
				s.logger.Debugf("Generator Worker %d 接收到取消信号。", id)
				return
			}
		}
	}
}

// scorerWorker 是负责评分和筛选密钥的 Worker
func (s *keyService) scorerWorker(id int, ctx context.Context, input <-chan *openpgp.Entity, output chan<- *models.KeyInfo, wg *sync.WaitGroup) {
	defer wg.Done()
	s.logger.Debugf("Scorer Worker %d 开始工作。", id)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debugf("Scorer Worker %d 接收到取消信号。", id)
			return
		case entity, ok := <-input:
			if !ok {
				s.logger.Debugf("Scorer Worker %d 完成工作。", id)
				return
			}

			fingerprint := fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)
			lastSixteen := fingerprint[len(fingerprint)-16:]
			scores, err := domain.CalculateScores(lastSixteen)
			if err != nil {
				s.logger.Errorf("Scorer Worker %d 计算分数失败: %v", id, err)
				continue
			}
			totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore + scores.DecreasingLetterScore + scores.MagicLetterScore

			// 不符合标准时跳过
			if totalScore <= s.config.MinScore || scores.UniqueLettersCount < s.config.MaxLettersCount {
				continue
			}

			pubKeyStr, privKeyStr, err := domain.SerializeKeys(entity, s.encryptor)
			if err != nil {
				s.logger.Errorf("Scorer Worker %d 序列化密钥失败: %v", id, err)
				continue
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

			select {
			case output <- keyInfo:
			case <-ctx.Done():
				s.logger.Debugf("Scorer Worker %d 接收到取消信号。", id)
				return
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

// ShowMinimalKeys 实现 ShowMinimalKeys 方法
func (s *keyService) ShowMinimalKeys(n int) error {
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
