package key

import (
	"crypto"
	"fmt"
	"strings"
	"time"

	"gpgenie/internal/config"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// NewEntity creates a new OpenPGP entity based on the configuration
func NewEntity(cfg *config.KeyGenerationConfig) (*openpgp.Entity, error) {
	return openpgp.NewEntity(cfg.Name, cfg.Comment, cfg.Email, &packet.Config{
		DefaultHash:     crypto.SHA256,
		Time:            time.Now,
		Algorithm:       packet.PubKeyAlgoEdDSA,
		KeyLifetimeSecs: 0,
	})
}

// SerializeKeys serializes the public and private keys into armored strings.
// If encryptor is provided, the private key is encrypted.
func SerializeKeys(entity *openpgp.Entity, encryptor *Encryptor) (string, string, error) {
	var pubKeyBuf, privKeyBuf strings.Builder

	// Serialize public key
	pubKeyArmor, err := armor.Encode(&pubKeyBuf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode public key: %w", err)
	}
	if err := entity.Serialize(pubKeyArmor); err != nil {
		return "", "", fmt.Errorf("failed to serialize public key: %w", err)
	}
	if err := pubKeyArmor.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close public key armor: %w", err)
	}

	// Serialize private key
	privKeyArmor, err := armor.Encode(&privKeyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode private key: %w", err)
	}
	if err := entity.SerializePrivate(privKeyArmor, nil); err != nil {
		return "", "", fmt.Errorf("failed to serialize private key: %w", err)
	}
	if err := privKeyArmor.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close private key armor: %w", err)
	}

	// Encrypt the private key if encryptor is available
	if encryptor != nil {
		encryptedPrivKey, err := encryptor.EncryptAndEncode(privKeyBuf.String())
		if err != nil {
			return "", "", fmt.Errorf("failed to encrypt private key: %w", err)
		}
		privKeyBuf.Reset()
		privKeyBuf.WriteString(encryptedPrivKey)
	}

	return pubKeyBuf.String(), privKeyBuf.String(), nil
}
