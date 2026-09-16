//go:build windows && (amd64 || arm64)

package vanity

import (
	"context"
	"fmt"
	"testing"
)

// Measure dispatch throughput separately from random key generation and driver
// compilation. Use -benchtime=1s -count=3 for stable comparisons.
func BenchmarkOpenCLDispatch(b *testing.B) {
	devices, err := listOpenCLDeviceRefs()
	if err != nil || len(devices) == 0 {
		b.Skipf("OpenCL GPU unavailable: %v", err)
	}
	for _, device := range devices {
		b.Run(device.Info.Name, func(b *testing.B) {
			const templates = 64
			const timestamps = 4096
			executor, err := newOpenCLExecutor(device, templates)
			if err != nil {
				b.Fatal(err)
			}
			defer executor.Close()
			words := make([]uint32, 0, templates*16)
			for range templates {
				_, template, err := generateCandidateKey()
				if err != nil {
					b.Fatal(err)
				}
				block, err := sha1PaddedBlock(template)
				if err != nil {
					b.Fatal(err)
				}
				words = append(words, block[:]...)
			}
			if err := executor.writeBlocks(words); err != nil {
				b.Fatal(err)
			}
			if _, err := executor.run(templates, 1700000000, timestamps, templates*timestamps,
				AllDigits, ScopeSuffix, 16, 8); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := executor.run(templates, 1700000000+uint32(i)*timestamps, timestamps,
					templates*timestamps, AllDigits, ScopeSuffix, 16, 8); err != nil {
					b.Fatal(err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N)*templates*timestamps/b.Elapsed().Seconds(), "hashes/s")
		})
	}
}

// End-to-end comparison including compilation, key generation, scheduling and
// verification. No files/database are written. Keep it explicitly bounded.
func BenchmarkSearchBackends(b *testing.B) {
	devices, err := ListOpenCLDevices()
	if err != nil || len(devices) == 0 {
		b.Skipf("OpenCL GPU unavailable: %v", err)
	}
	cases := []struct {
		name    string
		backend Backend
		workers int
		devices []int
	}{{"CPU_8", BackendCPU, 8, nil}, {"OpenCL_all", BackendOpenCL, 0, nil}}
	for _, workers := range []int{4, 8, 12, 16} {
		cases = append(cases, struct {
			name    string
			backend Backend
			workers int
			devices []int
		}{fmt.Sprintf("Hybrid_%d", workers), BackendHybrid, workers, nil})
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			const attempts = uint64(1_000_000_000)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := Search(context.Background(), SearchConfig{
					Backend: tc.backend, Workers: tc.workers, OpenCLDevices: tc.devices,
					MinRun: 16, Scope: ScopeSuffix, AllowedDigits: 0x103,
					TimestampStart: 1700000000, TimestampEnd: 1702591999, MaxAttempts: attempts,
				}, nil)
				if err != nil {
					b.Fatal(err)
				}
				if result.RunAttempts != attempts {
					b.Fatalf("attempts=%d, want=%d", result.RunAttempts, attempts)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(b.N)*float64(attempts)/b.Elapsed().Seconds(), "hashes/s")
		})
	}
}
