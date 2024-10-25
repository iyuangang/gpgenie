package domain

import (
	"bytes"
	"crypto"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gpgenie/internal/config"
	"gpgenie/internal/logger"
	"gpgenie/models"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// Score 结构体用于存储各项评分
type Score struct {
	RepeatLetterScore     int
	IncreasingLetterScore int
	DecreasingLetterScore int
	MagicLetterScore      int
	UniqueLettersCount    int
}

// NewEntity 生成一个新的 PGP 实体（密钥对）
func NewEntity(cfg config.KeyGenerationConfig) (*openpgp.Entity, error) {
	// 创建一个新的实体，包含用户信息
	entity, err := openpgp.NewEntity(cfg.Name, cfg.Comment, cfg.Email, &packet.Config{
		DefaultHash:     crypto.SHA256,
		Time:            time.Now,
		Algorithm:       packet.PubKeyAlgoEdDSA,
		KeyLifetimeSecs: 0,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create PGP entity: %w", err)
	}

	return entity, nil
}

// SerializeKeys 序列化公钥和私钥，并加密私钥
func SerializeKeys(entity *openpgp.Entity, encryptor Encryptor) (string, string, error) {
	// 序列化公钥
	var pubKeyBuf bytes.Buffer
	pubArmor, err := armor.Encode(&pubKeyBuf, openpgp.PublicKeyType, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create armor for public key: %w", err)
	}
	if err := entity.Serialize(pubArmor); err != nil {
		return "", "", fmt.Errorf("failed to serialize public key: %w", err)
	}
	if err := pubArmor.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close public armor: %w", err)
	}

	// 序列化私钥
	var privKeyBuf bytes.Buffer
	privArmor, err := armor.Encode(&privKeyBuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create armor for private key: %w", err)
	}
	if err := entity.SerializePrivate(privArmor, nil); err != nil {
		return "", "", fmt.Errorf("failed to serialize private key: %w", err)
	}
	if err := privArmor.Close(); err != nil {
		return "", "", fmt.Errorf("failed to close private armor: %w", err)
	}

	// 加密私钥
	encryptedPrivKey, err := encryptor.Encrypt(privKeyBuf.String())
	if err != nil {
		return "", "", fmt.Errorf("failed to encrypt private key: %w", err)
	}

	return pubKeyBuf.String(), encryptedPrivKey, nil
}

// DisplayKeys 格式化并显示密钥信息
func DisplayKeys(keys []models.KeyInfo) {
	for _, key := range keys {
		fmt.Printf("Fingerprint: %s\n", key.Fingerprint)
		fmt.Printf("Score: %d\n", key.Score)
		fmt.Printf("Unique Letters Count: %d\n", key.UniqueLettersCount)
		fmt.Println("-----")
	}
}

// ExportKey 导出密钥到指定目录
func ExportKey(key *models.KeyInfo, outputDir string, exportArmor bool, encryptor Encryptor, log *logger.Logger) error {
	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// 导出公钥
	pubKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s_pub.key", key.Fingerprint))
	if err := os.WriteFile(pubKeyPath, []byte(key.PublicKey), 0o600); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	// 导出私钥
	var privKeyData string
	if exportArmor {
		privKeyData = key.PrivateKey
	} else {
		// 如果不使用 ASCII Armor，加密私钥
		encryptedPrivKey, err := encryptor.Encrypt(key.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt private key: %w", err)
		}
		privKeyData = encryptedPrivKey
	}

	privKeyPath := filepath.Join(outputDir, fmt.Sprintf("%s_priv.key", key.Fingerprint))
	if err := os.WriteFile(privKeyPath, []byte(privKeyData), 0o600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	log.Infof("密钥已导出到 %s 和 %s", pubKeyPath, privKeyPath)
	return nil
}
