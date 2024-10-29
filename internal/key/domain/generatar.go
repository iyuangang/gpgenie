package domain

import (
	"fmt"

	"gpgenie/internal/config"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// GenerateKeyPair 生成一个密钥对
func GenerateKeyPair(cfg config.KeyGenerationConfig, encryptor Encryptor) (*openpgp.Entity, error) {
	entity, err := NewEntity(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	return entity, nil
}
