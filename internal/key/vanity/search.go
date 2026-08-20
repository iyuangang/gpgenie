package vanity

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const searchBatchSize = uint64(8192)

type Backend string

const (
	BackendCPU    Backend = "cpu"
	BackendOpenCL Backend = "opencl"
	BackendHybrid Backend = "hybrid"
	BackendAuto   Backend = "auto"
)

type SearchConfig struct {
	Backend          Backend
	Workers          int
	OpenCLDevices    []int
	GPUKeyBatch      int
	GPUWorkItems     uint64
	MinRun           int
	Scope            Scope
	AllowedDigits    DigitSet
	TimestampStart   uint32
	TimestampEnd     uint32
	MaxAttempts      uint64
	InitialAttempts  uint64
	InitialBestRun   int
	ProgressInterval time.Duration
}

type Candidate struct {
	Fingerprint [20]byte
	KeyID       uint64
	Timestamp   uint32
	Match       Match
	privateKey  *packet.PrivateKey
}

func (c Candidate) FingerprintHex() string {
	return fmt.Sprintf("%X", c.Fingerprint[:])
}

func (c Candidate) KeyIDHex() string {
	return fmt.Sprintf("%016X", c.KeyID)
}

func (c Candidate) RepeatedDigit() string {
	return formatHexDigit(c.Match.Digit)
}

type Progress struct {
	Attempts    uint64
	RunAttempts uint64
	BestRun     int
	BestKeyID   string
	Elapsed     time.Duration
	Rate        float64
	Final       bool
}

type SearchResult struct {
	Candidate     *Candidate
	Attempts      uint64
	RunAttempts   uint64
	BestRun       int
	Elapsed       time.Duration
	Rate          float64
	TargetReached bool
}

type ProgressFunc func(Progress)

func (c SearchConfig) validate() error {
	if c.Backend == "" {
		c.Backend = BackendCPU
	}
	if err := c.Backend.Validate(); err != nil {
		return err
	}
	if c.Workers < 0 || (c.Workers == 0 && c.Backend != BackendOpenCL) {
		return fmt.Errorf("workers must be greater than zero")
	}
	if c.GPUKeyBatch < 0 {
		return fmt.Errorf("GPU key batch must not be negative")
	}
	if c.GPUKeyBatch > maxGPUKeyBatch {
		return fmt.Errorf("GPU key batch must not exceed %d", maxGPUKeyBatch)
	}
	if c.GPUWorkItems > maxGPUWorkItems {
		return fmt.Errorf("GPU work items must not exceed %d", maxGPUWorkItems)
	}
	if c.MinRun < 1 || c.MinRun > 16 {
		return fmt.Errorf("min run must be between 1 and 16")
	}
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if c.TimestampStart > c.TimestampEnd {
		return fmt.Errorf("timestamp start must not be after timestamp end")
	}
	if c.ProgressInterval < 0 {
		return fmt.Errorf("progress interval must not be negative")
	}
	return nil
}

func Search(ctx context.Context, cfg SearchConfig, progressFn ProgressFunc) (*SearchResult, error) {
	if cfg.Backend == "" {
		cfg.Backend = BackendCPU
	}
	if cfg.Workers == 0 {
		cfg.Workers = runtime.NumCPU()
	}
	if cfg.AllowedDigits == 0 {
		cfg.AllowedDigits = AllDigits
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	effectiveBackend, openCLDevices, err := resolveBackendRefs(cfg.Backend, cfg.OpenCLDevices)
	if err != nil {
		return nil, err
	}
	cfg.Backend = effectiveBackend

	startedAt := time.Now()
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var completed atomic.Uint64
	var reserved atomic.Uint64
	var bestRun atomic.Int32
	bestRun.Store(int32(cfg.InitialBestRun))

	runnerCount := 0
	if effectiveBackend == BackendCPU || effectiveBackend == BackendHybrid {
		runnerCount += cfg.Workers
	}
	if effectiveBackend == BackendOpenCL || effectiveBackend == BackendHybrid {
		runnerCount += len(openCLDevices)
	}
	candidates := make(chan Candidate, max(2, runnerCount*2))
	errorsCh := make(chan error, 1)
	var workers sync.WaitGroup
	startRunner := func(name string, run func() error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := run(); err != nil {
				select {
				case errorsCh <- fmt.Errorf("%s: %w", name, err):
				default:
				}
				cancel()
			}
		}()
	}
	if effectiveBackend == BackendCPU || effectiveBackend == BackendHybrid {
		for workerID := 0; workerID < cfg.Workers; workerID++ {
			id := workerID
			startRunner(fmt.Sprintf("CPU worker %d", id), func() error {
				return searchWorker(searchCtx, cfg, &completed, &reserved, &bestRun, candidates)
			})
		}
	}
	if effectiveBackend == BackendOpenCL || effectiveBackend == BackendHybrid {
		for _, device := range openCLDevices {
			device := device
			startRunner(fmt.Sprintf("OpenCL device %d (%s)", device.Info.Index, device.Info.Name), func() error {
				return searchOpenCLWorker(searchCtx, cfg, device, &completed, &reserved, &bestRun, candidates)
			})
		}
	}
	go func() {
		workers.Wait()
		close(candidates)
	}()

	interval := cfg.ProgressInterval
	if interval == 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var best *Candidate
	var firstErr error
	emitProgress := func(final bool) {
		if progressFn == nil {
			return
		}
		runAttempts := completed.Load()
		elapsed := time.Since(startedAt)
		rate := 0.0
		if elapsed > 0 {
			rate = float64(runAttempts) / elapsed.Seconds()
		}
		keyID := ""
		if best != nil {
			keyID = best.KeyIDHex()
		}
		progressFn(Progress{
			Attempts:    cfg.InitialAttempts + runAttempts,
			RunAttempts: runAttempts,
			BestRun:     int(bestRun.Load()),
			BestKeyID:   keyID,
			Elapsed:     elapsed,
			Rate:        rate,
			Final:       final,
		})
	}

	for {
		select {
		case candidate, ok := <-candidates:
			if !ok {
				emitProgress(true)
				runAttempts := completed.Load()
				elapsed := time.Since(startedAt)
				rate := 0.0
				if elapsed > 0 {
					rate = float64(runAttempts) / elapsed.Seconds()
				}
				result := &SearchResult{
					Candidate:     best,
					Attempts:      cfg.InitialAttempts + runAttempts,
					RunAttempts:   runAttempts,
					BestRun:       int(bestRun.Load()),
					Elapsed:       elapsed,
					Rate:          rate,
					TargetReached: best != nil && best.Match.RunLength >= cfg.MinRun,
				}
				if best != nil && best.Match.RunLength != result.BestRun {
					return result, fmt.Errorf("best candidate run %d does not match promoted run %d", best.Match.RunLength, result.BestRun)
				}
				if firstErr != nil {
					return result, firstErr
				}
				if ctx.Err() != nil && !result.TargetReached {
					return result, ctx.Err()
				}
				return result, nil
			}
			if best == nil || candidate.Match.RunLength > best.Match.RunLength {
				copyCandidate := candidate
				best = &copyCandidate
			}
			if candidate.Match.RunLength >= cfg.MinRun {
				cancel()
			}
		case err := <-errorsCh:
			if err != nil && firstErr == nil {
				firstErr = err
			}
		case <-ticker.C:
			emitProgress(false)
		}
	}
}

func searchWorker(
	ctx context.Context,
	cfg SearchConfig,
	completed *atomic.Uint64,
	reserved *atomic.Uint64,
	bestRun *atomic.Int32,
	output chan<- Candidate,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		privateKey, template, err := generateCandidateKey()
		if err != nil {
			return err
		}

		cursor := uint64(cfg.TimestampStart)
		end := uint64(cfg.TimestampEnd)
		for cursor <= end {
			if err := ctx.Err(); err != nil {
				return nil
			}
			remaining := end - cursor + 1
			chunk := minUint64(searchBatchSize, remaining)
			claimed := claimAttempts(reserved, cfg.MaxAttempts, chunk)
			if claimed == 0 {
				return nil
			}

			processed := uint64(0)
			for processed < claimed {
				if processed&1023 == 0 {
					select {
					case <-ctx.Done():
						completed.Add(processed)
						return nil
					default:
					}
				}

				timestamp := uint32(cursor + processed)
				fingerprint, keyID, err := fingerprintAt(template, timestamp)
				if err != nil {
					completed.Add(processed)
					return err
				}
				match := EvaluateKeyIDForDigits(keyID, cfg.Scope, cfg.AllowedDigits)
				if promoteBest(bestRun, match.RunLength) {
					candidate := Candidate{
						Fingerprint: fingerprint,
						KeyID:       keyID,
						Timestamp:   timestamp,
						Match:       match,
						privateKey:  privateKey,
					}
					// Once promoted, a candidate must reach the coordinator even if a
					// different worker has already caused search cancellation. This
					// keeps the reported best run and the retained private key aligned.
					output <- candidate
					if match.RunLength >= cfg.MinRun {
						completed.Add(processed + 1)
						return nil
					}
				}
				processed++
			}
			completed.Add(processed)
			cursor += claimed
		}
	}
}

func claimAttempts(reserved *atomic.Uint64, maximum, requested uint64) uint64 {
	if maximum == 0 {
		return requested
	}
	for {
		current := reserved.Load()
		if current >= maximum {
			return 0
		}
		claim := minUint64(requested, maximum-current)
		if reserved.CompareAndSwap(current, current+claim) {
			return claim
		}
	}
}

func promoteBest(best *atomic.Int32, candidate int) bool {
	for {
		current := best.Load()
		if int32(candidate) <= current {
			return false
		}
		if best.CompareAndSwap(current, int32(candidate)) {
			return true
		}
	}
}

func minUint64(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
