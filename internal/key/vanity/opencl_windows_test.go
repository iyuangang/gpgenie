//go:build windows && (amd64 || arm64)

package vanity

import (
	"context"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetentionSafeGPUWorkItems(t *testing.T) {
	assert.Equal(t, uint64(1024), retentionSafeGPUWorkItems(defaultGPUWorkItems, 1, ScopeSuffix))
	assert.Equal(t, uint64(16384), retentionSafeGPUWorkItems(defaultGPUWorkItems, 2, ScopeSuffix))
	assert.Equal(t, uint64(1024), retentionSafeGPUWorkItems(defaultGPUWorkItems, 2, ScopeAny))
	assert.Equal(t, defaultGPUWorkItems, retentionSafeGPUWorkItems(defaultGPUWorkItems, 12, ScopeSuffix))
}

func TestTuneGPUWorkItems(t *testing.T) {
	assert.Equal(t, uint64(20000), tuneGPUWorkItems(10000, time.Millisecond, 100000))
	assert.Equal(t, uint64(10000), tuneGPUWorkItems(10000, 100*time.Millisecond, 100000))
	assert.Equal(t, uint64(4096), tuneGPUWorkItems(10000, time.Second, 100000))
	assert.Equal(t, uint64(17), tuneGPUWorkItems(17, time.Second, 17))
	assert.Equal(t, uint64(10000), tuneGPUWorkItems(10000, 0, 100000))
	assert.Equal(t, uint64(100000), tuneGPUWorkItems(90000, time.Nanosecond, 100000))
}

func TestOpenCLAllScoresAndHitsAgainstCPU(t *testing.T) {
	devices, err := listOpenCLDeviceRefs()
	if err != nil || len(devices) == 0 {
		t.Skipf("OpenCL GPU unavailable: %v", err)
	}
	for _, device := range devices {
		t.Run(device.Info.Name, func(t *testing.T) {
			const templates = 3
			executor, err := newOpenCLExecutor(device, templates)
			require.NoError(t, err)
			defer executor.Close()
			var inputs [][]byte
			var words []uint32
			for range templates {
				_, template, err := generateCandidateKey()
				require.NoError(t, err)
				inputs = append(inputs, template)
				block, err := sha1PaddedBlock(template)
				require.NoError(t, err)
				words = append(words, block[:]...)
			}
			require.NoError(t, executor.writeBlocks(words))
			for _, scope := range []Scope{ScopeSuffix, ScopeAny} {
				for _, allowed := range []DigitSet{AllDigits, 0x103, 0x8000} {
					for _, start := range []uint32{0, 0xfffffeff} {
						// 257 timestamps exercises workgroup padding; workCount
						// truncates the last template, including max uint32 time.
						for _, minRun := range []int{1, 2, 4, 16} {
							for _, baseline := range []int{0, 3, 16} {
								const timestamps, work = 257, 769
								result, err := executor.run(templates, start, timestamps, work, allowed, scope, minRun, baseline)
								require.NoError(t, err)
								want := map[uint32]bool{}
								best := uint32(baseline) << openCLResultIndexBits
								for index := uint32(0); index < work; index++ {
									_, keyID, err := fingerprintAt(inputs[index/timestamps], start+index%timestamps)
									require.NoError(t, err)
									run := referenceMatch(keyID, scope, allowed).RunLength
									score := uint32(run)<<openCLResultIndexBits | index
									if run > baseline {
										best = max(best, score)
									}
									if run >= minRun {
										want[score] = true
									}
								}
								require.Equal(t, best, result[0])
								require.Equal(t, len(want), int(result[1]))
								for _, score := range result[2 : 2+result[1]] {
									require.True(t, want[score], "unexpected or duplicate GPU hit %x", score)
									delete(want, score)
								}
								require.Empty(t, want)
							}
						}
					}
				}
			}
		})
	}
}

// Supply synthetic key IDs directly to the same OpenCL matching function so
// that rare runs of lengths 8-16 and every start position are also exercised.
func TestOpenCLMatcherLongRuns(t *testing.T) {
	devices, err := listOpenCLDeviceRefs()
	if err != nil || len(devices) == 0 {
		t.Skipf("OpenCL GPU unavailable: %v", err)
	}
	rng := rand.New(rand.NewPCG(6, 7))
	var keyIDs []uint64
	var words []uint32
	for run := 1; run <= 16; run++ {
		for start := 0; start <= 16-run; start++ {
			for digit := uint64(0); digit < 16; digit++ {
				shift := uint(start * 4)
				mask := ^uint64(0) >> uint((16-run)*4)
				keyID := rng.Uint64() & ^(mask<<shift) | (digit*0x1111111111111111&mask)<<shift
				keyIDs = append(keyIDs, keyID)
				block := [16]uint32{uint32(keyID >> 32), uint32(keyID)}
				words = append(words, block[:]...)
			}
		}
	}
	source := strings.Split(openCLKernelHeader, "__kernel")[0] + `
__kernel void vanity_sha1(__global const uint *blocks, uint template_count,
    uint timestamp_start, uint timestamp_count, uint work_count, uint allowed,
    uint scope, uint min_run, volatile __global uint *result, uint initial_best) {
    if (get_global_id(0) != 0) return;
    uint i = (uint)get_global_id(1);
    uint run = matching_run(blocks[i * 16], blocks[i * 16 + 1], scope, allowed);
    result[2 + i] = run;
    if (i == 0) result[1] = template_count;
}`
	for _, device := range devices {
		t.Run(device.Info.Name, func(t *testing.T) {
			executor, err := newOpenCLExecutorSource(device, len(keyIDs), source)
			require.NoError(t, err)
			defer executor.Close()
			require.NoError(t, executor.writeBlocks(words))
			for _, scope := range []Scope{ScopeSuffix, ScopeAny} {
				for _, digits := range []DigitSet{AllDigits, 0x103, 0x8000} {
					result, err := executor.run(uint32(len(keyIDs)), 0, 1, uint32(len(keyIDs)), digits, scope, 1, 0)
					require.NoError(t, err)
					for i, keyID := range keyIDs {
						require.Equal(t, referenceMatch(keyID, scope, digits).RunLength, int(result[2+i]), "key=%016x", keyID)
					}
				}
			}
		})
	}
}

func TestOpenCLSmallBudgetAndBatchLargerThanDispatch(t *testing.T) {
	devices, err := ListOpenCLDevices()
	if err != nil || len(devices) == 0 {
		t.Skipf("OpenCL GPU unavailable: %v", err)
	}
	for _, device := range devices {
		result, err := Search(context.Background(), SearchConfig{
			Backend: BackendOpenCL, OpenCLDevices: []int{device.Index},
			GPUKeyBatch: 4096, GPUWorkItems: 7, MinRun: 1, Scope: ScopeSuffix,
			TimestampStart: 0xfffffffe, TimestampEnd: 0xffffffff, MaxAttempts: 17,
		}, nil)
		require.NoError(t, err)
		require.Equal(t, uint64(17), result.RunAttempts)
		require.Equal(t, uint64(17), result.Retained)
	}
}
