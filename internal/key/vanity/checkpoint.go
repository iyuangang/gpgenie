package vanity

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Checkpoint struct {
	Attempts                   uint64 `json:"attempts"`
	BestRun                    int    `json:"best_run"`
	Scope                      Scope  `json:"scope,omitempty"`
	TargetDigits               string `json:"target_digits,omitempty"`
	BestKeyID                  string `json:"best_key_id,omitempty"`
	BestSigningFingerprint     string `json:"best_signing_fingerprint,omitempty"`
	LatestPublicKeyPath        string `json:"latest_public_key_path,omitempty"`
	LatestEncryptedPrivatePath string `json:"latest_encrypted_private_path,omitempty"`
	LatestMetadataPath         string `json:"latest_metadata_path,omitempty"`
	SavedToDatabase            bool   `json:"saved_to_database,omitempty"`
	UpdatedAt                  string `json:"updated_at"`
}

func LoadCheckpoint(path string) (*Checkpoint, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Checkpoint{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read checkpoint: %w", err)
	}
	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("parse checkpoint: %w", err)
	}
	return &checkpoint, nil
}

func SaveCheckpoint(path string, checkpoint Checkpoint) error {
	if path == "" {
		return nil
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve checkpoint path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o700); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}
	checkpoint.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(absPath, data, 0o600); err != nil {
		return fmt.Errorf("write checkpoint: %w", err)
	}
	return nil
}
