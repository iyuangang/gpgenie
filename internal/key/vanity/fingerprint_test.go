package vanity

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprintAtMatchesOpenPGPV4Serialization(t *testing.T) {
	privateKey, template, err := generateCandidateKey()
	require.NoError(t, err)
	timestamp := uint32(time.Now().Add(-24 * time.Hour).Unix())

	fingerprint, keyID, err := fingerprintAt(template, timestamp)
	require.NoError(t, err)

	privateKey.CreationTime = time.Unix(int64(timestamp), 0)
	var material bytes.Buffer
	require.NoError(t, privateKey.PublicKey.SerializeForHash(&material))
	wantFingerprint := sha1.Sum(material.Bytes())

	assert.Equal(t, wantFingerprint, fingerprint)
	assert.Equal(t, binary.BigEndian.Uint64(wantFingerprint[12:]), keyID)
}

func TestFingerprintTemplateRejectsNil(t *testing.T) {
	_, err := newFingerprintTemplate(nil)
	require.Error(t, err)
}

var benchmarkKeyID atomic.Uint64

func BenchmarkFingerprintAndSuffixMatch(b *testing.B) {
	_, template, err := generateCandidateKey()
	if err != nil {
		b.Fatal(err)
	}
	baseTimestamp := uint32(time.Now().Add(-24 * time.Hour).Unix())

	b.ReportAllocs()
	b.ResetTimer()
	var lastKeyID uint64
	for i := 0; i < b.N; i++ {
		_, keyID, err := fingerprintAt(template, baseTimestamp+uint32(i))
		if err != nil {
			b.Fatal(err)
		}
		lastKeyID = keyID
		_ = EvaluateKeyID(keyID, ScopeSuffix)
	}
	benchmarkKeyID.Store(lastKeyID)
}

func BenchmarkFingerprintAndSuffixMatchParallel(b *testing.B) {
	_, originalTemplate, err := generateCandidateKey()
	if err != nil {
		b.Fatal(err)
	}
	baseTimestamp := uint32(time.Now().Add(-24 * time.Hour).Unix())

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		template := append([]byte(nil), originalTemplate...)
		timestamp := baseTimestamp
		var lastKeyID uint64
		for pb.Next() {
			_, keyID, err := fingerprintAt(template, timestamp)
			if err != nil {
				b.Fatal(err)
			}
			lastKeyID = keyID
			_ = EvaluateKeyID(keyID, ScopeSuffix)
			timestamp++
		}
		benchmarkKeyID.Store(lastKeyID)
	})
}
