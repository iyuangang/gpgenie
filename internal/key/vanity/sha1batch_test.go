package vanity

import (
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyIDsBatchMatchesStandardSHA1(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	// Include every valid single-block length, timestamp carry/wrap boundaries,
	// odd tails and empty output. Padding and SIMD lanes must agree exactly.
	for length := 8; length <= 55; length++ {
		template := make([]byte, length)
		for i := range template {
			template[i] = byte(rng.Uint32())
		}
		template[0], template[3] = 0x99, 4
		block, err := sha1PaddedBlock(template)
		require.NoError(t, err)
		for _, start := range []uint32{0, 0xfe, 0xfffe, 0xfffffe, 1700000000, 0xfffffffe} {
			for _, count := range []int{0, 1, 2, 3, 16, 257, 1024} {
				output := make([]uint64, count+2)
				output[0], output[count+1] = 0xdeadbeef, 0x12345678
				keyIDsBatch(&block, template, start, output[1:count+1])
				for i, got := range output[1 : count+1] {
					_, want, err := fingerprintAt(template, start+uint32(i))
					require.NoError(t, err)
					if got != want {
						t.Fatalf("length=%d start=%x count=%d lane=%d: got %016x want %016x", length, start, count, i, got, want)
					}
				}
				require.Equal(t, uint64(0xdeadbeef), output[0])
				require.Equal(t, uint64(0x12345678), output[count+1])
			}
		}
	}
}

func BenchmarkKeyIDsBatch(b *testing.B) {
	_, template, err := generateCandidateKey()
	if err != nil {
		b.Fatal(err)
	}
	block, err := sha1PaddedBlock(template)
	if err != nil {
		b.Fatal(err)
	}
	var output [1024]uint64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		keyIDsBatch(&block, template, uint32(i)*1024, output[:])
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)*1024/b.Elapsed().Seconds(), "hashes/s")
	benchmarkKeyID.Store(output[0])
}
