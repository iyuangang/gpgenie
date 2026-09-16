//go:build !amd64 || purego

package vanity

func keyIDsBatch(_ *[16]uint32, template []byte, timestamp uint32, output []uint64) {
	keyIDsGeneric(template, timestamp, output)
}
