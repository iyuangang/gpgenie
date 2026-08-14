package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iyuangang/gpgenie/internal/config"
	"github.com/iyuangang/gpgenie/internal/key/domain"
	"github.com/iyuangang/gpgenie/internal/logger"
	"github.com/iyuangang/gpgenie/internal/repository"
	"github.com/iyuangang/gpgenie/models"

	"github.com/ProtonMail/go-crypto/openpgp"
)

const pipelineBufferMultiplier = 2

// KeyService defines the interface for the key service.
type KeyService interface {
	GenerateKeys(ctx context.Context) error
	ShowTopKeys(n int) error
	ShowMinimalKeys(n int) error
	ExportKeyByFingerprint(lastSixteen, outputDir string, exportArmor bool) error
	AnalyzeData() error
}

type keyService struct {
	repo      repository.KeyRepository
	config    *config.KeyGenerationConfig
	encryptor domain.Encryptor
	logger    *logger.Logger
}

// NewKeyService creates a key service. The configuration is kept by reference so
// command-line overrides applied before GenerateKeys are honored.
func NewKeyService(repo repository.KeyRepository, cfg *config.KeyGenerationConfig, encryptor domain.Encryptor, log *logger.Logger) KeyService {
	return &keyService{
		repo:      repo,
		config:    cfg,
		encryptor: encryptor,
		logger:    log,
	}
}

// GenerateKeys runs a bounded generate -> score -> persist pipeline.
func (s *keyService) GenerateKeys(parent context.Context) error {
	if parent == nil {
		parent = context.Background()
	}

	cfg := *s.config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid key generation configuration: %w", err)
	}

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	startedAt := time.Now()

	generationJobs := make(chan struct{}, cfg.NumGeneratorWorkers*pipelineBufferMultiplier)
	generatedEntities := make(chan *openpgp.Entity, (cfg.NumGeneratorWorkers+cfg.NumScorerWorkers)*pipelineBufferMultiplier)
	scoredKeyInfos := make(chan *models.KeyInfo, cfg.NumScorerWorkers*pipelineBufferMultiplier)

	var (
		producerWG   sync.WaitGroup
		generatorWG  sync.WaitGroup
		scorerWG     sync.WaitGroup
		firstErr     error
		firstErrOnce sync.Once
		generated    atomic.Uint64
		accepted     atomic.Uint64
	)

	fail := func(err error) {
		if err == nil {
			return
		}
		firstErrOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	producerWG.Add(1)
	go func() {
		defer producerWG.Done()
		defer close(generationJobs)
		for i := 0; i < cfg.TotalKeys; i++ {
			select {
			case generationJobs <- struct{}{}:
			case <-ctx.Done():
				return
			}
		}
	}()

	for i := 0; i < cfg.NumGeneratorWorkers; i++ {
		generatorWG.Add(1)
		go s.generatorWorker(i, ctx, cfg, generationJobs, generatedEntities, &generatorWG, &generated, fail)
	}
	go func() {
		generatorWG.Wait()
		close(generatedEntities)
	}()

	for i := 0; i < cfg.NumScorerWorkers; i++ {
		workerEncryptor, err := cloneEncryptor(s.encryptor)
		if err != nil {
			fail(fmt.Errorf("initialize scorer worker %d encryptor: %w", i, err))
			break
		}
		scorerWG.Add(1)
		go s.scorerWorker(i, ctx, cfg, workerEncryptor, generatedEntities, scoredKeyInfos, &scorerWG, &accepted, fail)
	}
	go func() {
		scorerWG.Wait()
		close(scoredKeyInfos)
	}()

	saved, persistErr := s.persistBatches(ctx, scoredKeyInfos, cfg.BatchSize)
	if persistErr != nil {
		fail(persistErr)
	}

	producerWG.Wait()
	generatorWG.Wait()
	scorerWG.Wait()

	if firstErr != nil {
		return firstErr
	}
	if err := parent.Err(); err != nil {
		return err
	}

	elapsed := time.Since(startedAt)
	rate := 0.0
	if elapsed > 0 {
		rate = float64(generated.Load()) / elapsed.Seconds()
	}
	s.logger.Infof(
		"Key generation completed: generated=%d accepted=%d saved=%d elapsed=%s rate=%.2f candidates/s.",
		generated.Load(), accepted.Load(), saved, elapsed.Round(time.Millisecond), rate,
	)
	return nil
}

func (s *keyService) persistBatches(ctx context.Context, input <-chan *models.KeyInfo, batchSize int) (uint64, error) {
	batch := make([]*models.KeyInfo, 0, batchSize)
	var saved uint64
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		s.logger.Debugf("Saving %d keys to database.", len(batch))
		if err := s.repo.BatchCreate(batch); err != nil {
			return fmt.Errorf("save key batch: %w", err)
		}
		saved += uint64(len(batch))
		batch = batch[:0]
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return saved, ctx.Err()
		case keyInfo, ok := <-input:
			if !ok {
				if err := flush(); err != nil {
					return saved, err
				}
				return saved, nil
			}
			if keyInfo == nil {
				continue
			}
			batch = append(batch, keyInfo)
			if len(batch) >= batchSize {
				if err := flush(); err != nil {
					return saved, err
				}
			}
		}
	}
}

func (s *keyService) generatorWorker(
	id int,
	ctx context.Context,
	cfg config.KeyGenerationConfig,
	jobs <-chan struct{},
	output chan<- *openpgp.Entity,
	wg *sync.WaitGroup,
	generated *atomic.Uint64,
	fail func(error),
) {
	defer wg.Done()
	s.logger.Debugf("Generator Worker %d started.", id)

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-jobs:
			if !ok {
				return
			}
			entity, err := domain.GenerateKeyPair(cfg)
			if err != nil {
				fail(fmt.Errorf("generator worker %d: %w", id, err))
				return
			}
			select {
			case output <- entity:
				generated.Add(1)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *keyService) scorerWorker(
	id int,
	ctx context.Context,
	cfg config.KeyGenerationConfig,
	encryptor domain.Encryptor,
	input <-chan *openpgp.Entity,
	output chan<- *models.KeyInfo,
	wg *sync.WaitGroup,
	accepted *atomic.Uint64,
	fail func(error),
) {
	defer wg.Done()
	s.logger.Debugf("Scorer Worker %d started.", id)

	for {
		select {
		case <-ctx.Done():
			return
		case entity, ok := <-input:
			if !ok {
				return
			}

			fingerprint := hex.EncodeToString(entity.PrimaryKey.Fingerprint[:])
			lastSixteen := fingerprint[len(fingerprint)-16:]
			scores, err := domain.CalculateScores(lastSixteen)
			if err != nil {
				fail(fmt.Errorf("scorer worker %d: calculate score: %w", id, err))
				return
			}

			totalScore := scores.RepeatLetterScore + scores.IncreasingLetterScore +
				scores.DecreasingLetterScore + scores.MagicLetterScore
			if totalScore <= cfg.MinScore && scores.UniqueLettersCount > cfg.MaxLettersCount {
				continue
			}

			pubKey, privateKey, err := domain.SerializeKeys(entity, encryptor)
			if err != nil {
				fail(fmt.Errorf("scorer worker %d: serialize keys: %w", id, err))
				return
			}

			keyInfo := &models.KeyInfo{
				Fingerprint:           fingerprint,
				FingerprintSuffix:     lastSixteen,
				PublicKey:             pubKey,
				PrivateKey:            privateKey,
				RepeatLetterScore:     scores.RepeatLetterScore,
				IncreasingLetterScore: scores.IncreasingLetterScore,
				DecreasingLetterScore: scores.DecreasingLetterScore,
				MagicLetterScore:      scores.MagicLetterScore,
				Score:                 totalScore,
				UniqueLettersCount:    scores.UniqueLettersCount,
			}

			select {
			case output <- keyInfo:
				accepted.Add(1)
			case <-ctx.Done():
				return
			}
		}
	}
}

func (s *keyService) ShowTopKeys(n int) error {
	if n <= 0 {
		return fmt.Errorf("count must be greater than zero")
	}
	keys, err := s.repo.GetTopKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve top keys: %w", err)
	}
	domain.DisplayKeys(keys)
	return nil
}

func (s *keyService) ShowMinimalKeys(n int) error {
	if n <= 0 {
		return fmt.Errorf("count must be greater than zero")
	}
	keys, err := s.repo.GetLowLetterCountKeys(n)
	if err != nil {
		return fmt.Errorf("failed to retrieve low letter count keys: %w", err)
	}
	domain.DisplayKeys(keys)
	return nil
}

func (s *keyService) ExportKeyByFingerprint(lastSixteen, outputDir string, exportArmor bool) error {
	keyInfo, err := s.repo.GetByFingerprint(lastSixteen)
	if err != nil {
		return fmt.Errorf("failed to find key: %w", err)
	}
	return domain.ExportKey(keyInfo, outputDir, exportArmor, s.encryptor, s.logger)
}

func (s *keyService) AnalyzeData() error {
	analyzer := domain.NewAnalyzer(s.repo)
	return analyzer.PerformAnalysis()
}
