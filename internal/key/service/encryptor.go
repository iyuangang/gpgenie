package service

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// PGPEncryptor is the concrete implementation of the Encryptor interface using OpenPGP for encryption
type PGPEncryptor struct {
	entity *openpgp.Entity
	armor  *armor.Block
	mu     sync.Mutex
}

// NewPGPEncryptor creates a new PGPEncryptor instance
func NewPGPEncryptor(publicKeyPath string) (*PGPEncryptor, error) {
	pubKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pubKeyData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no public key found in the provided public key file")
	}

	return &PGPEncryptor{entity: entities[0]}, nil
}

// Encrypt implements the Encryptor interface method, returning the encrypted string
func (e *PGPEncryptor) Encrypt(plaintext string) (string, error) {
	var buf bytes.Buffer

	e.mu.Lock()
	defer e.mu.Unlock()

	armorWriter, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return "", fmt.Errorf("failed to initialize Armor encoder: %w", err)
	}

	writer, err := openpgp.Encrypt(armorWriter, []*openpgp.Entity{e.entity}, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt: %w", err)
	}

	_, err = writer.Write([]byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("failed to write encrypted data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close encrypt writer: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return "", fmt.Errorf("failed to close Armor encoder: %w", err)
	}

	return buf.String(), nil
}
