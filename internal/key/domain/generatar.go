package domain

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"time"

	"gpgenie/internal/config"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// GenerateKeyPair 生成一个密钥对
func GenerateKeyPair(cfg config.KeyGenerationConfig, encryptor Encryptor) (*openpgp.Entity, error) {
	entity, err := NewEntity(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create entity: %w", err)
	}

	return entity, nil
}

// generateBareKeyPair 生成裸ED25519密钥对
func GenerateBareKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
  // 1. 直接生成ED25519密钥对
  pub, priv, err := ed25519.GenerateKey(rand.Reader)
  if err != nil {
      return nil, nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
  }

  return pub, priv, nil
}

func PackPrivateKey(pub ed25519.PublicKey, priv ed25519.PrivateKey) (*openpgp.Entity, error) {
  currentTime := time.Now()

  // 3. 构造私钥包
  privateKey := &packet.PrivateKey{
      PublicKey: packet.PublicKey{
          CreationTime: currentTime,
          PubKeyAlgo:   packet.PubKeyAlgoEdDSA,
          PublicKey:    pub,
      },
      PrivateKey: priv,
  }

  // 4. 创建实体
  entity := &openpgp.Entity{
      PrimaryKey: &privateKey.PublicKey,
      PrivateKey: privateKey,
      Identities: make(map[string]*openpgp.Identity),
      Subkeys:    make([]openpgp.Subkey, 0),
  }

  return entity, nil
}
