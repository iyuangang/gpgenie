package service

import (
	"bytes"
	"fmt"
	"os"
	"sync"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
)

// PGPEncryptor 是 Encryptor 接口的具体实现，使用 OpenPGP 进行加密
type PGPEncryptor struct {
	entity *openpgp.Entity
	armor  *armor.Block
	mu     sync.Mutex
}

// NewPGPEncryptor 创建一个新的 PGPEncryptor 实例
func NewPGPEncryptor(publicKeyPath string) (*PGPEncryptor, error) {
	pubKeyData, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("无法读取公钥文件: %w", err)
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(pubKeyData))
	if err != nil {
		return nil, fmt.Errorf("无法解析公钥: %w", err)
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("提供的公钥文件中未找到公钥")
	}

	return &PGPEncryptor{entity: entities[0]}, nil
}

// Encrypt 实现 Encryptor 接口的方法，返回加密后的字符串
func (e *PGPEncryptor) Encrypt(plaintext string) (string, error) {
	var buf bytes.Buffer

	e.mu.Lock()
	defer e.mu.Unlock()

	armorWriter, err := armor.Encode(&buf, "PGP MESSAGE", nil)
	if err != nil {
		return "", fmt.Errorf("初始化 Armor 编码器失败: %w", err)
	}

	writer, err := openpgp.Encrypt(armorWriter, []*openpgp.Entity{e.entity}, nil, nil, nil)
	if err != nil {
		return "", fmt.Errorf("加密失败: %w", err)
	}

	_, err = writer.Write([]byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("写入加密数据失败: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("关闭加密写入器失败: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return "", fmt.Errorf("关闭 Armor 编码器失败: %w", err)
	}

	return buf.String(), nil
}
