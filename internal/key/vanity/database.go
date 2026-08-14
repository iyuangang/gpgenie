package vanity

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/iyuangang/gpgenie/internal/key/domain"
	"github.com/iyuangang/gpgenie/models"
)

// LoadArtifacts reloads a finalized vanity result. This is used to retry an
// optional database save without repeating the expensive search.
func LoadArtifacts(publicPath, encryptedPrivatePath, metadataPath string) (*Artifacts, error) {
	publicKey, err := os.ReadFile(publicPath)
	if err != nil {
		return nil, fmt.Errorf("read vanity public key: %w", err)
	}
	encryptedPrivateKey, err := os.ReadFile(encryptedPrivatePath)
	if err != nil {
		return nil, fmt.Errorf("read encrypted vanity private key: %w", err)
	}
	metadataJSON, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read vanity metadata: %w", err)
	}

	var metadata ArtifactMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("parse vanity metadata: %w", err)
	}
	return &Artifacts{
		PublicKeyPath:        publicPath,
		EncryptedPrivatePath: encryptedPrivatePath,
		MetadataPath:         metadataPath,
		Metadata:             metadata,
		PublicKey:            string(publicKey),
		EncryptedPrivateKey:  string(encryptedPrivateKey),
	}, nil
}

// ToDatabaseKeyInfo converts finalized artifacts into the existing encrypted
// key record format. Fingerprint identifies the signing subkey because that is
// the fingerprint Git and GitHub use for commit signatures.
func (a *Artifacts) ToDatabaseKeyInfo() (*models.KeyInfo, error) {
	if a == nil {
		return nil, fmt.Errorf("vanity artifacts are nil")
	}
	metadata := a.Metadata
	signingFingerprint := strings.ToLower(metadata.SigningSubkeyFingerprint)
	signingKeyID := strings.ToLower(metadata.SigningKeyID)
	primaryFingerprint := strings.ToLower(metadata.PrimaryFingerprint)
	if !validHexLength(signingFingerprint, 40) {
		return nil, fmt.Errorf("invalid signing subkey fingerprint %q", metadata.SigningSubkeyFingerprint)
	}
	if !validHexLength(signingKeyID, 16) || !strings.HasSuffix(signingFingerprint, signingKeyID) {
		return nil, fmt.Errorf("invalid signing key ID %q", metadata.SigningKeyID)
	}
	if !validHexLength(primaryFingerprint, 40) {
		return nil, fmt.Errorf("invalid primary fingerprint %q", metadata.PrimaryFingerprint)
	}
	if metadata.RunLength < 1 || metadata.RunLength > 16 {
		return nil, fmt.Errorf("invalid vanity run length %d", metadata.RunLength)
	}
	if strings.TrimSpace(a.PublicKey) == "" {
		return nil, fmt.Errorf("vanity public key is empty")
	}
	if strings.TrimSpace(a.EncryptedPrivateKey) == "" {
		return nil, fmt.Errorf("encrypted vanity private key is empty")
	}

	scores, err := domain.CalculateScores(signingKeyID)
	if err != nil {
		return nil, fmt.Errorf("calculate vanity key scores: %w", err)
	}
	return &models.KeyInfo{
		Fingerprint:           signingFingerprint,
		FingerprintSuffix:     signingKeyID,
		PrimaryFingerprint:    primaryFingerprint,
		PublicKey:             a.PublicKey,
		PrivateKey:            a.EncryptedPrivateKey,
		RepeatLetterScore:     scores.RepeatLetterScore,
		IncreasingLetterScore: scores.IncreasingLetterScore,
		DecreasingLetterScore: scores.DecreasingLetterScore,
		MagicLetterScore:      scores.MagicLetterScore,
		Score: scores.RepeatLetterScore + scores.IncreasingLetterScore +
			scores.DecreasingLetterScore + scores.MagicLetterScore,
		UniqueLettersCount: scores.UniqueLettersCount,
		IsVanity:           true,
		VanityRunLength:    metadata.RunLength,
		VanityRunStart:     metadata.RunStart,
		VanityDigit:        metadata.RepeatedDigit,
		VanityScope:        string(metadata.Scope),
		VanityTargetDigits: metadata.TargetDigits,
	}, nil
}

func validHexLength(value string, length int) bool {
	if len(value) != length {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
