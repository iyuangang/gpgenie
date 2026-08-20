package vanity

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	openpgpEdDSA "github.com/ProtonMail/go-crypto/openpgp/eddsa"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

var candidateCurve struct {
	sync.Once
	publicKey *openpgpEdDSA.PublicKey
	err       error
}

// generateCandidateKey creates only the Ed25519 material needed by a vanity
// signing subkey. The ProtonMail API does not export an Ed25519 curve
// constructor, so one prototype is initialized once; subsequent candidates
// use crypto/ed25519 directly and avoid NewEntity's user ID, signatures, and
// encryption subkey generation.
func generateCandidateKey() (*packet.PrivateKey, []byte, error) {
	candidateCurve.Do(func() {
		createdAt := time.Unix(1, 0).UTC()
		entity, err := openpgp.NewEntity("GPGenie Candidate", "", "candidate@gpgenie.invalid", &packet.Config{
			DefaultHash: crypto.SHA256,
			Time:        func() time.Time { return createdAt },
			Algorithm:   packet.PubKeyAlgoEdDSA,
		})
		if err != nil {
			candidateCurve.err = fmt.Errorf("initialize Ed25519 curve: %w", err)
			return
		}
		privateKey, ok := entity.PrivateKey.PrivateKey.(*openpgpEdDSA.PrivateKey)
		if !ok {
			candidateCurve.err = fmt.Errorf("unexpected Ed25519 private key type %T", entity.PrivateKey.PrivateKey)
			return
		}
		candidateCurve.publicKey = &privateKey.PublicKey
	})
	if candidateCurve.err != nil {
		return nil, nil, candidateCurve.err
	}

	publicBytes, privateBytes, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate Ed25519 candidate: %w", err)
	}
	publicKey := openpgpEdDSA.NewPublicKey(candidateCurve.publicKey.GetCurve())
	publicKey.X = append([]byte(nil), publicBytes...)
	privateKey := openpgpEdDSA.NewPrivateKey(*publicKey)
	privateKey.D = append([]byte(nil), privateBytes.Seed()...)

	createdAt := time.Now().UTC().Truncate(time.Second)
	packetKey := packet.NewEdDSAPrivateKey(createdAt, privateKey)
	template, err := newFingerprintTemplate(&packetKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return packetKey, template, nil
}
