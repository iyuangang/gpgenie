package vanity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/iyuangang/gpgenie/internal/key/domain"
)

type ArtifactMetadata struct {
	PrimaryFingerprint       string  `json:"primary_fingerprint"`
	SigningSubkeyFingerprint string  `json:"signing_subkey_fingerprint"`
	SigningKeyID             string  `json:"signing_key_id"`
	Scope                    Scope   `json:"scope"`
	TargetDigits             string  `json:"target_digits"`
	RunLength                int     `json:"run_length"`
	RunStart                 int     `json:"run_start"`
	RepeatedDigit            string  `json:"repeated_digit"`
	SubkeyCreatedAt          string  `json:"subkey_created_at"`
	PrimaryCreatedAt         string  `json:"primary_created_at"`
	Attempts                 uint64  `json:"attempts"`
	RunAttempts              uint64  `json:"run_attempts"`
	Elapsed                  string  `json:"elapsed"`
	Rate                     float64 `json:"candidates_per_second"`
	CreatedAt                string  `json:"created_at"`
}

type Artifacts struct {
	PublicKeyPath        string
	EncryptedPrivatePath string
	MetadataPath         string
	Metadata             ArtifactMetadata
	PublicKey            string
	EncryptedPrivateKey  string
}

func FinalizeAndWrite(
	outputDir string,
	identity Identity,
	candidate Candidate,
	primaryCreatedAt time.Time,
	searchResult *SearchResult,
	scope Scope,
	targetDigits string,
	encryptor domain.Encryptor,
) (*Artifacts, error) {
	if encryptor == nil {
		return nil, fmt.Errorf("encryptor is nil")
	}
	if searchResult == nil {
		return nil, fmt.Errorf("search result is nil")
	}

	entity, err := BuildSigningKeyring(identity, candidate, primaryCreatedAt)
	if err != nil {
		return nil, err
	}
	publicKey, encryptedPrivateKey, err := domain.SerializeKeys(entity, encryptor)
	if err != nil {
		return nil, fmt.Errorf("serialize vanity keyring: %w", err)
	}

	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("resolve output directory: %w", err)
	}
	if err := os.MkdirAll(absOutputDir, 0o700); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	keyID := candidate.KeyIDHex()
	baseName := "gpgenie-" + keyID
	publicPath := filepath.Join(absOutputDir, baseName+"-public.asc")
	privatePath := filepath.Join(absOutputDir, baseName+"-private.asc.pgp")
	metadataPath := filepath.Join(absOutputDir, baseName+"-result.json")

	metadata := ArtifactMetadata{
		PrimaryFingerprint:       fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint),
		SigningSubkeyFingerprint: candidate.FingerprintHex(),
		SigningKeyID:             keyID,
		Scope:                    scope,
		TargetDigits:             targetDigits,
		RunLength:                candidate.Match.RunLength,
		RunStart:                 candidate.Match.Start,
		RepeatedDigit:            candidate.RepeatedDigit(),
		SubkeyCreatedAt:          time.Unix(int64(candidate.Timestamp), 0).UTC().Format(time.RFC3339),
		PrimaryCreatedAt:         primaryCreatedAt.UTC().Format(time.RFC3339),
		Attempts:                 searchResult.Attempts,
		RunAttempts:              searchResult.RunAttempts,
		Elapsed:                  searchResult.Elapsed.Round(time.Millisecond).String(),
		Rate:                     searchResult.Rate,
		CreatedAt:                time.Now().UTC().Format(time.RFC3339),
	}
	metadataJSON, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode result metadata: %w", err)
	}
	metadataJSON = append(metadataJSON, '\n')

	if err := os.WriteFile(publicPath, []byte(publicKey), 0o644); err != nil {
		return nil, fmt.Errorf("write public key: %w", err)
	}
	if err := os.WriteFile(privatePath, []byte(encryptedPrivateKey), 0o600); err != nil {
		return nil, fmt.Errorf("write encrypted private key: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataJSON, 0o600); err != nil {
		return nil, fmt.Errorf("write result metadata: %w", err)
	}

	return &Artifacts{
		PublicKeyPath:        publicPath,
		EncryptedPrivatePath: privatePath,
		MetadataPath:         metadataPath,
		Metadata:             metadata,
		PublicKey:            publicKey,
		EncryptedPrivateKey:  encryptedPrivateKey,
	}, nil
}
