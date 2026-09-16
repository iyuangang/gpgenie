package vanity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchRetainsEveryQualifiedCandidateAndContinues(t *testing.T) {
	now := uint32(time.Now().Unix())
	var retained []Candidate
	result, err := Search(context.Background(), SearchConfig{
		Workers:        1,
		MinRun:         1,
		Scope:          ScopeSuffix,
		TimestampStart: now - 10,
		TimestampEnd:   now,
		MaxAttempts:    100,
		OnCandidate: func(candidate Candidate, _ Progress) error {
			retained = append(retained, candidate)
			return nil
		},
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, result.Candidate)
	assert.True(t, result.TargetReached)
	assert.Equal(t, uint64(100), result.RunAttempts)
	assert.Equal(t, uint64(100), result.Retained)
	assert.Len(t, retained, 100)
	for _, candidate := range retained {
		assert.GreaterOrEqual(t, candidate.Match.RunLength, 1)
	}
	assert.Equal(t, result.BestRun, result.Candidate.Match.RunLength)
}

func TestSearchHonorsAttemptBudget(t *testing.T) {
	now := uint32(time.Now().Unix())
	result, err := Search(context.Background(), SearchConfig{
		Workers:        2,
		MinRun:         16,
		Scope:          ScopeSuffix,
		TimestampStart: now - 100,
		TimestampEnd:   now,
		MaxAttempts:    1000,
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, uint64(1000), result.RunAttempts)
	assert.False(t, result.TargetReached)
	if result.Candidate != nil {
		assert.Equal(t, result.BestRun, result.Candidate.Match.RunLength)
	}
}

func TestSearchRetainsBestCandidateDuringConcurrentSearch(t *testing.T) {
	now := uint32(time.Now().Unix())
	for i := 0; i < 10; i++ {
		result, err := Search(context.Background(), SearchConfig{
			Workers:        8,
			MinRun:         2,
			Scope:          ScopeSuffix,
			TimestampStart: now - 1000,
			TimestampEnd:   now,
			MaxAttempts:    10000,
		}, nil)
		require.NoError(t, err)
		require.NotNil(t, result.Candidate)
		assert.Equal(t, result.BestRun, result.Candidate.Match.RunLength)
	}
}

func TestSearchRetainsQualifiedCandidateBelowCheckpointBest(t *testing.T) {
	now := uint32(time.Now().Unix())
	retained := 0
	result, err := Search(context.Background(), SearchConfig{
		Workers:         1,
		MinRun:          1,
		Scope:           ScopeSuffix,
		TimestampStart:  now - 20,
		TimestampEnd:    now,
		MaxAttempts:     20,
		InitialBestRun:  16,
		InitialRetained: 4,
		OnCandidate: func(candidate Candidate, progress Progress) error {
			assert.GreaterOrEqual(t, candidate.Match.RunLength, 1)
			assert.Greater(t, progress.Retained, uint64(4))
			retained++
			return nil
		},
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, 20, retained)
	assert.Equal(t, uint64(24), result.Retained)
	assert.Equal(t, 16, result.BestRun)
	assert.Nil(t, result.Candidate)
}

func TestSearchStopsOnRetentionError(t *testing.T) {
	now := uint32(time.Now().Unix())
	wantErr := errors.New("disk full")
	result, err := Search(context.Background(), SearchConfig{
		Workers:        2,
		MinRun:         1,
		Scope:          ScopeSuffix,
		TimestampStart: now - 100,
		TimestampEnd:   now,
		MaxAttempts:    1000,
		OnCandidate: func(Candidate, Progress) error {
			return wantErr
		},
	}, nil)

	require.ErrorIs(t, err, wantErr)
	require.NotNil(t, result)
	assert.Zero(t, result.Retained)
}

func TestSearchCancellationDrainsComputedCandidates(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var retained uint64
	result, err := Search(ctx, SearchConfig{
		Workers: 8, MinRun: 1, Scope: ScopeAny,
		TimestampStart: 0xffffff00, TimestampEnd: 0xffffffff,
		OnCandidate: func(_ Candidate, _ Progress) error {
			retained++
			cancel()
			return nil
		},
	}, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, retained, result.Retained)
	require.Equal(t, retained, result.RunAttempts)
	require.Positive(t, retained)
}

func TestOpenCLSearchMatchesCPUVerification(t *testing.T) {
	devices, err := ListOpenCLDevices()
	if err != nil || len(devices) == 0 {
		t.Skipf("OpenCL GPU unavailable: %v", err)
	}

	now := uint32(time.Now().Unix())
	for _, device := range devices {
		device := device
		t.Run(device.Name, func(t *testing.T) {
			result, err := Search(context.Background(), SearchConfig{
				Backend:        BackendOpenCL,
				OpenCLDevices:  []int{device.Index},
				GPUKeyBatch:    2,
				GPUWorkItems:   4096,
				MinRun:         16,
				Scope:          ScopeSuffix,
				TimestampStart: now - 4095,
				TimestampEnd:   now,
				MaxAttempts:    8192,
			}, nil)

			require.NoError(t, err)
			assert.Equal(t, uint64(8192), result.RunAttempts)
			require.NotNil(t, result.Candidate)
			assert.Equal(t, result.BestRun, result.Candidate.Match.RunLength)
			assert.False(t, result.TargetReached)

			retained := 0
			continuous, err := Search(context.Background(), SearchConfig{
				Backend:        BackendOpenCL,
				OpenCLDevices:  []int{device.Index},
				GPUKeyBatch:    2,
				GPUWorkItems:   256,
				MinRun:         1,
				Scope:          ScopeSuffix,
				TimestampStart: now - 127,
				TimestampEnd:   now,
				MaxAttempts:    256,
				OnCandidate: func(candidate Candidate, _ Progress) error {
					assert.GreaterOrEqual(t, candidate.Match.RunLength, 1)
					retained++
					return nil
				},
			}, nil)
			require.NoError(t, err)
			assert.Equal(t, 256, retained)
			assert.Equal(t, uint64(256), continuous.Retained)
		})
	}
}

func BenchmarkSearchTenMillionCandidates(b *testing.B) {
	now := uint32(time.Now().Unix())
	const attemptsPerRun = uint64(10_000_000)
	b.SetBytes(int64(attemptsPerRun))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := Search(context.Background(), SearchConfig{
			Workers:        10,
			MinRun:         16,
			Scope:          ScopeSuffix,
			TimestampStart: now - uint32(attemptsPerRun),
			TimestampEnd:   now,
			MaxAttempts:    attemptsPerRun,
		}, nil)
		if err != nil {
			b.Fatal(err)
		}
		if result.RunAttempts != attemptsPerRun {
			b.Fatalf("processed %d attempts, want %d", result.RunAttempts, attemptsPerRun)
		}
	}
}
