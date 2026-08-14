package vanity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchStopsWhenTargetReached(t *testing.T) {
	now := uint32(time.Now().Unix())
	result, err := Search(context.Background(), SearchConfig{
		Workers:        1,
		MinRun:         1,
		Scope:          ScopeSuffix,
		TimestampStart: now - 10,
		TimestampEnd:   now,
		MaxAttempts:    100,
	}, nil)

	require.NoError(t, err)
	require.NotNil(t, result.Candidate)
	assert.True(t, result.TargetReached)
	assert.Equal(t, 1, result.Candidate.Match.RunLength)
	assert.Equal(t, result.BestRun, result.Candidate.Match.RunLength)
	assert.GreaterOrEqual(t, result.RunAttempts, uint64(1))
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

func TestSearchRetainsEveryPromotedCandidateDuringConcurrentCancellation(t *testing.T) {
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
