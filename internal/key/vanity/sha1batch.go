package vanity

import (
	"encoding/binary"
	"fmt"
)

//go:generate go run ./internal/gensha1

// sha1PaddedBlock prepares the invariant part once per random public key.
// Both CPU SIMD and OpenCL consume host-order words, varying only word 1.
func sha1PaddedBlock(template []byte) ([16]uint32, error) {
	var result [16]uint32
	if len(template) > 55 || len(template) < v4TimestampOffset+4 || template[0] != 0x99 || template[v4VersionOffset] != 4 {
		return result, fmt.Errorf("SHA-1 fast path requires a valid v4 fingerprint template of 8-55 bytes, got %d", len(template))
	}
	var block [64]byte
	copy(block[:], template)
	block[len(template)] = 0x80
	binary.BigEndian.PutUint64(block[56:], uint64(len(template))*8)
	for i := range result {
		result[i] = binary.BigEndian.Uint32(block[i*4 : i*4+4])
	}
	return result, nil
}

// The generic path retains the standard library's architecture-specific
// implementation. Full fingerprints are reconstructed only for actual hits.
func keyIDsGeneric(template []byte, timestamp uint32, output []uint64) {
	for i := range output {
		_, output[i], _ = fingerprintAt(template, timestamp+uint32(i))
	}
}
