package vanity

import (
	"bytes"
	"crypto"
	"fmt"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

type Identity struct {
	Name    string
	Comment string
	Email   string
}

// BuildSigningKeyring creates a normal Ed25519 primary key and binds the
// mined candidate as a cross-certified Ed25519 signing subkey.
func BuildSigningKeyring(identity Identity, candidate Candidate, primaryCreatedAt time.Time) (*openpgp.Entity, error) {
	if candidate.privateKey == nil {
		return nil, fmt.Errorf("candidate private key is missing")
	}
	if identity.Name == "" || identity.Email == "" {
		return nil, fmt.Errorf("name and email are required")
	}

	subkeyCreatedAt := time.Unix(int64(candidate.Timestamp), 0).UTC()
	primaryCreatedAt = primaryCreatedAt.UTC().Truncate(time.Second)
	if !primaryCreatedAt.Before(subkeyCreatedAt) {
		return nil, fmt.Errorf("primary key creation time must be before signing subkey creation time")
	}

	primaryConfig := &packet.Config{
		DefaultHash: crypto.SHA256,
		Time:        func() time.Time { return primaryCreatedAt },
		Algorithm:   packet.PubKeyAlgoEdDSA,
	}
	entity, err := openpgp.NewEntity(identity.Name, identity.Comment, identity.Email, primaryConfig)
	if err != nil {
		return nil, fmt.Errorf("generate primary key: %w", err)
	}
	// GitHub commit signing does not require the automatically generated
	// encryption subkey. Keep the transferable key focused on certification
	// and the mined signing subkey.
	entity.Subkeys = nil
	for _, identity := range entity.Identities {
		if identity.SelfSignature == nil {
			return nil, fmt.Errorf("primary identity is missing its self-signature")
		}
		identity.SelfSignature.FlagsValid = true
		identity.SelfSignature.FlagCertify = true
		identity.SelfSignature.FlagSign = false
		if err := identity.SelfSignature.SignUserId(
			identity.UserId.Id,
			entity.PrimaryKey,
			entity.PrivateKey,
			primaryConfig,
		); err != nil {
			return nil, fmt.Errorf("restrict primary key to certification: %w", err)
		}
	}

	subPrivate := candidate.privateKey
	subPrivate.Version = 4
	subPrivate.CreationTime = subkeyCreatedAt
	subPrivate.IsSubkey = true
	subPrivate.Fingerprint = append([]byte(nil), candidate.Fingerprint[:]...)
	subPrivate.KeyId = candidate.KeyID
	subPublic := &subPrivate.PublicKey

	verificationTemplate, err := newFingerprintTemplate(subPublic)
	if err != nil {
		return nil, err
	}
	verifiedFingerprint, verifiedKeyID, err := fingerprintAt(verificationTemplate, candidate.Timestamp)
	if err != nil {
		return nil, err
	}
	if verifiedFingerprint != candidate.Fingerprint || verifiedKeyID != candidate.KeyID {
		return nil, fmt.Errorf("candidate fingerprint changed while constructing signing subkey")
	}

	bindingTime := time.Now().UTC().Truncate(time.Second)
	if !bindingTime.After(subkeyCreatedAt) {
		bindingTime = subkeyCreatedAt.Add(time.Second)
	}
	signConfig := &packet.Config{
		DefaultHash: crypto.SHA256,
		Time:        func() time.Time { return bindingTime },
	}

	binding := newSignature(entity.PrimaryKey, packet.SigTypeSubkeyBinding, bindingTime)
	keyLifetime := uint32(0)
	binding.KeyLifetimeSecs = &keyLifetime
	binding.FlagsValid = true
	binding.FlagSign = true

	embedded := newSignature(subPublic, packet.SigTypePrimaryKeyBinding, bindingTime)
	if err := embedded.CrossSignKey(subPublic, entity.PrimaryKey, subPrivate, signConfig); err != nil {
		return nil, fmt.Errorf("cross-sign vanity signing subkey: %w", err)
	}
	binding.EmbeddedSignature = embedded
	if err := binding.SignKey(subPublic, entity.PrivateKey, signConfig); err != nil {
		return nil, fmt.Errorf("bind vanity signing subkey: %w", err)
	}

	entity.Subkeys = []openpgp.Subkey{{
		PublicKey:  subPublic,
		PrivateKey: subPrivate,
		Sig:        binding,
	}}
	if err := ValidateSigningKeyring(entity, candidate.KeyID); err != nil {
		return nil, err
	}
	return entity, nil
}

func newSignature(signer *packet.PublicKey, signatureType packet.SignatureType, createdAt time.Time) *packet.Signature {
	issuerKeyID := signer.KeyId
	signatureLifetime := uint32(0)
	return &packet.Signature{
		Version:           signer.Version,
		SigType:           signatureType,
		PubKeyAlgo:        signer.PubKeyAlgo,
		Hash:              crypto.SHA256,
		CreationTime:      createdAt,
		IssuerKeyId:       &issuerKeyID,
		IssuerFingerprint: append([]byte(nil), signer.Fingerprint...),
		SigLifetimeSecs:   &signatureLifetime,
	}
}

func ValidateSigningKeyring(entity *openpgp.Entity, signingKeyID uint64) error {
	if entity == nil || entity.PrimaryKey == nil {
		return fmt.Errorf("keyring is incomplete")
	}
	if len(entity.Subkeys) != 1 {
		return fmt.Errorf("expected one signing subkey, got %d", len(entity.Subkeys))
	}
	subkey := entity.Subkeys[0]
	if subkey.PublicKey.KeyId != signingKeyID {
		return fmt.Errorf("unexpected signing subkey ID: got %016X, want %016X", subkey.PublicKey.KeyId, signingKeyID)
	}
	if err := entity.PrimaryKey.VerifyKeySignature(subkey.PublicKey, subkey.Sig); err != nil {
		return fmt.Errorf("verify signing subkey binding: %w", err)
	}

	now := time.Now().UTC()
	selected, ok := entity.SigningKeyById(now, signingKeyID)
	if !ok || selected.PublicKey.KeyId != signingKeyID {
		return fmt.Errorf("vanity signing subkey is not selectable")
	}
	// A public-only keyring can validate the binding and key selection but
	// cannot perform the private signing round trip below.
	if entity.PrivateKey == nil || selected.PrivateKey == nil {
		return nil
	}

	message := []byte("gpgenie vanity signing key validation")
	var signature bytes.Buffer
	config := &packet.Config{
		DefaultHash:  crypto.SHA256,
		SigningKeyId: signingKeyID,
		Time:         func() time.Time { return now },
	}
	if err := openpgp.DetachSign(&signature, entity, bytes.NewReader(message), config); err != nil {
		return fmt.Errorf("sign validation message: %w", err)
	}
	signaturePacket, _, err := openpgp.VerifyDetachedSignature(
		openpgp.EntityList{entity},
		bytes.NewReader(message),
		bytes.NewReader(signature.Bytes()),
		config,
	)
	if err != nil {
		return fmt.Errorf("verify validation signature: %w", err)
	}
	if signaturePacket.IssuerKeyId == nil || *signaturePacket.IssuerKeyId != signingKeyID {
		return fmt.Errorf("validation signature was not produced by the vanity signing subkey")
	}
	return nil
}
