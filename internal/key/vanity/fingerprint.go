package vanity

import (
	"bytes"
	"crypto/sha1" // OpenPGP v4 fingerprints require SHA-1 by specification.
	"encoding/binary"
	"fmt"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

const (
	v4FingerprintPrefixLength = 3
	v4VersionOffset           = v4FingerprintPrefixLength
	v4TimestampOffset         = v4VersionOffset + 1
)

func newFingerprintTemplate(publicKey *packet.PublicKey) ([]byte, error) {
	if publicKey == nil {
		return nil, fmt.Errorf("public key is nil")
	}
	if publicKey.Version != 4 {
		return nil, fmt.Errorf("only OpenPGP version 4 keys are supported, got version %d", publicKey.Version)
	}

	var material bytes.Buffer
	if err := publicKey.SerializeForHash(&material); err != nil {
		return nil, fmt.Errorf("serialize public key for fingerprint: %w", err)
	}
	data := material.Bytes()
	if len(data) < v4TimestampOffset+4 || data[0] != 0x99 || data[v4VersionOffset] != 4 {
		return nil, fmt.Errorf("unexpected OpenPGP v4 fingerprint material")
	}
	return append([]byte(nil), data...), nil
}

func fingerprintAt(template []byte, timestamp uint32) ([sha1.Size]byte, uint64, error) {
	if len(template) < v4TimestampOffset+4 || template[0] != 0x99 || template[v4VersionOffset] != 4 {
		return [sha1.Size]byte{}, 0, fmt.Errorf("invalid OpenPGP v4 fingerprint template")
	}
	binary.BigEndian.PutUint32(template[v4TimestampOffset:v4TimestampOffset+4], timestamp)
	fingerprint := sha1.Sum(template)
	keyID := binary.BigEndian.Uint64(fingerprint[sha1.Size-8:])
	return fingerprint, keyID, nil
}
