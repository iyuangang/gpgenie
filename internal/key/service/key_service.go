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

// KeyService defines the interface for the key service
type KeyService interface {
	GenerateKeys(ctx context.Context) error
	ShowTopKeys(n int) error
	ShowMinimalKeys(n int) error
	ExportKeyByFingerprint(lastSixteen, outputDir string, exportArmor bool) error
	AnalyzeData() error
}

// keyService is the implementation of the KeyService interface
type keyService struct {
	repo      repository.KeyRepository
	config    config.KeyGenerationConfig
	encryptor domain.Encryptor
	logger    *logger.Logger
}

// NewKeyService creates a new KeyService instance, passing in the Encryptor interface
func NewKeyService(repo repository.KeyRepository, cfg config.KeyGenerationConfig, encryptor domain.Encryptor, log *logger.Logger) KeyService {
	return &keyService{
		repo:      repo,
		config:    cfg,
		encryptor: encryptor,
		logger:    log,
	}
}

// GenerateKeys generates key pairs
func (s *keyService) GenerateKeys(ctx context.Context) error {
	cfg := s.config
	generatorWorkerCount := cfg.NumGeneratorWorkers
	scorerWorkerCount := cfg.NumScorerWorkers
	jobCount := cfg.TotalKeys

	// Channels for pipeline
	generationJobs := make(chan struct{}, generatorWorkerCount*20)
	generatedEntities := make(chan *openpgp.Entity, jobCount)
	scoredKeyInfos := make(chan *models.KeyInfo, scorerWorkerCount*20)

	// Create KeyInfo object pool
	keyInfoPool := sync.Pool{
		New: func() interface{} {
			return &models.KeyInfo{}
		},
	}

	var (
		wgGenerators sync.WaitGroup
		wgScorers    sync.WaitGroup
		errChan      = make(chan error, 1)
	)

	// Start Generator Workers
	for i := 0; i < generatorWorkerCount; i++ {
		wgGenerators.Add(1)
		go s.generatorWorker(i, ctx, generationJobs, generatedEntities, &wgGenerators)
		s.logger.Debugf("Generator Worker %d started.", i)
	}

	// Start Scorer Workers with object pool
	for i := 0; i < scorerWorkerCount; i++ {
		wgScorers.Add(1)
		go s.scorerWorker(i, ctx, generatedEntities, scoredKeyInfos, &wgScorers, &keyInfoPool)
		s.logger.Debugf("Scorer Worker %d started.", i)
	}

	// Optimized Saver Goroutine
	insertWg := sync.WaitGroup{}
	insertWg.Add(1)
	go func() {
		defer insertWg.Done()

		localBatch := make([]*models.KeyInfo, 0, cfg.BatchSize)
		tx := s.repo.BeginTransaction()
		defer func() {
			if r := recover(); r != nil {
				if err := tx.Rollback(); err != nil {
					select {
					case errChan <- fmt.Errorf("rollback transaction failed: %v", err):
					default:
					}
				}
			}
		}()

		for keyInfo := range scoredKeyInfos {
			if keyInfo == nil {
				continue
			}

			localBatch = append(localBatch, keyInfo)
			if len(localBatch) >= cfg.BatchSize {
				s.logger.Infof("Saving %d keys to database.", len(localBatch))
				if err := tx.BatchCreate(localBatch); err != nil {
					select {
					case errChan <- err:
					default:
					}
					return
				}
				// Reset batch and preallocate capacity
				localBatch = make([]*models.KeyInfo, 0, cfg.BatchSize)
			}
		}

		// Process remaining keys
		if len(localBatch) > 0 {
			s.logger.Infof("Saving %d keys to database.", len(localBatch))
			if err := tx.BatchCreate(localBatch); err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
		}

		if err := tx.Commit(); err != nil {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Start Enqueue Generation Jobs
	go func() {
		defer close(generationJobs)
		for i := 0; i < jobCount; i++ {
			select {
			case generationJobs <- struct{}{}:
			case <-ctx.Done():
				s.logger.Warn("Enqueue Generation Jobs canceled.")
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

	s.logger.Info("Key generation process completed.")
	return nil
}

// generatorWorker is a worker for generating key pairs
func (s *keyService) generatorWorker(id int, ctx context.Context, jobs <-chan struct{}, output chan<- *openpgp.Entity, wg *sync.WaitGroup) {
	defer wg.Done()
	s.logger.Debugf("Generator Worker %d start working.", id)
	for {
		select {
		case <-ctx.Done():
			s.logger.Debugf("Generator Worker %d received cancel signal.", id)
			return
		case _, ok := <-jobs:
			if !ok {
				s.logger.Debugf("Generator Worker %d finished working.", id)
				return
			}
			entity, err := domain.GenerateKeyPair(s.config, s.encryptor)
			if err != nil {
				s.logger.Errorf("Generator Worker %d generate key failed: %v", id, err)
				continue
			}
			select {
			case output <- entity:
			case <-ctx.Done():
				s.logger.Debugf("Generator Worker %d received cancel signal.", id)
				return
			}
		}
	}
}

// scorerWorker is a worker for scoring and filtering key pairs
func (s *keyService) scorerWorker(id int, ctx context.Context, input <-chan *openpgp.Entity,
	output chan<- *models.KeyInfo, wg *sync.WaitGroup, pool *sync.Pool,
) {
	defer wg.Done()
	s.logger.Debugf("Scorer Worker %d start working.", id)

	for {
		select {
		case <-ctx.Done():
			s.logger.Debugf("Scorer Worker %d received cancel signal.", id)
			return
		case entity, ok := <-input:
			if !ok {
				s.logger.Debugf("Scorer Worker %d finished working.", id)
				return
			}

			fingerprint := fmt.Sprintf("%x", entity.PrimaryKey.Fingerprint)
			lastSixteen := fingerprint[len(fingerprint)-16:]

			scores, err := domain.CalculateScores(lastSixteen)
			if err != nil {
				s.logger.Errorf("Scorer Worker %d calculate scores failed: %v", id, err)
				continue
			}

			totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore +
				scores.DecreasingLetterScore + scores.MagicLetterScore

			if totalScore <= s.config.MinScore && scores.UniqueLettersCount > s.config.MaxLettersCount {
				continue
			}

			pubKeyStr, privKeyStr, err := domain.SerializeKeys(entity, s.encryptor)
			if err != nil {
				s.logger.Errorf("Scorer Worker %d serialize keys failed: %v", id, err)
				continue
			}

			// Get KeyInfo from pool
			keyInfo := pool.Get().(*models.KeyInfo)
			keyInfo.Fingerprint = fingerprint
			keyInfo.PublicKey = pubKeyStr
			keyInfo.PrivateKey = privKeyStr
			keyInfo.RepeatLetterScore = scores.RepeatLetterScore
			keyInfo.IncreasingLetterScore = scores.IncreasingLetterScore
			keyInfo.DecreasingLetterScore = scores.DecreasingLetterScore
			keyInfo.MagicLetterScore = scores.MagicLetterScore
			keyInfo.Score = totalScore
			keyInfo.UniqueLettersCount = scores.UniqueLettersCount

			select {
			case output <- keyInfo:
			case <-ctx.Done():
				pool.Put(keyInfo)
				s.logger.Debugf("Scorer Worker %d received cancel signal.", id)
				return
			}
		}
	}
}

// ShowTopKeys implements the ShowTopKeys method
func (s *keyService) ShowTopKeys(n int) error {
	keys, err := s.repo.GetTopKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve top keys: %w", err)
	}

	domain.DisplayKeys(keys)
	return nil
}

// ShowMinimalKeys implements the ShowMinimalKeys method
func (s *keyService) ShowMinimalKeys(n int) error {
	keys, err := s.repo.GetLowLetterCountKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve low letter count keys: %w", err)
	}

	domain.DisplayKeys(keys)
	return nil
}

// ExportKeyByFingerprint implements the ExportKeyByFingerprint method
func (s *keyService) ExportKeyByFingerprint(lastSixteen, outputDir string, exportArmor bool) error {
	keyInfo, err := s.repo.GetByFingerprint(lastSixteen)
	if err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}

	return domain.ExportKey(keyInfo, outputDir, exportArmor, s.encryptor, s.logger)
}

// AnalyzeData implements the AnalyzeData method
func (s *keyService) AnalyzeData() error {
	analyzer := domain.NewAnalyzer(s.repo)
	return analyzer.PerformAnalysis()
}
