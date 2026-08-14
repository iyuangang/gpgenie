package service

import (
	"bytes"
	"fmt"
	"os"

	"github.com/iyuangang/gpgenie/internal/key/domain"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

const armoredPublicKeyHeader = "-----BEGIN PGP PUBLIC KEY BLOCK-----"

// PGPEncryptor is the concrete implementation of the Encryptor interface using OpenPGP for encryption
type PGPEncryptor struct {
	entity        *openpgp.Entity
	publicKeyData []byte
}

// NewPGPEncryptor creates a new PGPEncryptor instance
func NewPGPEncryptor(publicKeyPath string) (*PGPEncryptor, error) {
	pubKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file: %w", err)
	}

	return newPGPEncryptor(pubKeyData)
}

func newPGPEncryptor(publicKeyData []byte) (*PGPEncryptor, error) {
	trimmedKey := bytes.TrimSpace(publicKeyData)
	if len(trimmedKey) == 0 {
		return nil, fmt.Errorf("public key file is empty; expected an ASCII-armored OpenPGP public key beginning with %q", armoredPublicKeyHeader)
	}

	if isOpenSSHPublicKey(trimmedKey) {
		return nil, fmt.Errorf("configured key is an OpenSSH public key; GPGenie requires an ASCII-armored OpenPGP public key beginning with %q (do not use a .ssh/*.pub key)", armoredPublicKeyHeader)
	}

	if !bytes.HasPrefix(trimmedKey, []byte(armoredPublicKeyHeader)) {
		return nil, fmt.Errorf("unsupported public key format; expected an ASCII-armored OpenPGP public key beginning with %q", armoredPublicKeyHeader)
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(publicKeyData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("no public key found in the provided public key file")
	}

	return &PGPEncryptor{
		entity:        entities[0],
		publicKeyData: append([]byte(nil), publicKeyData...),
	}, nil
}

func isOpenSSHPublicKey(publicKeyData []byte) bool {
	return bytes.HasPrefix(publicKeyData, []byte("ssh-")) ||
		bytes.HasPrefix(publicKeyData, []byte("ecdsa-")) ||
		bytes.HasPrefix(publicKeyData, []byte("sk-"))
}

// Clone creates a worker-local parsed entity so encryption workers do not share
// mutable OpenPGP state.
func (e *PGPEncryptor) Clone() (domain.Encryptor, error) {
	return newPGPEncryptor(e.publicKeyData)
}

// Encrypt implements the Encryptor interface method, returning the encrypted string
func (e *PGPEncryptor) Encrypt(plaintext string) (string, error) {
	var buf bytes.Buffer

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

func cloneEncryptor(encryptor domain.Encryptor) (domain.Encryptor, error) {
	if cloneable, ok := encryptor.(domain.CloneableEncryptor); ok {
		return cloneable.Clone()
	}
	return encryptor, nil
}
