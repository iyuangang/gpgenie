package domain

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// CalculateFingerprint 计算公钥的 SHA256 指纹
func CalculateFingerprint(pubKey *packet.PublicKey) (string, error) {
	// 预分配足够大的缓冲区
	buf := bytes.NewBuffer(make([]byte, 0, 128))

	// 1. 写入包头
	// Version 4
	if err := buf.WriteByte(0x04); err != nil {
		return "", fmt.Errorf("failed to write version: %w", err)
	}

	// 2. 写入创建时间
	timeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(timeBytes, uint32(pubKey.CreationTime.Unix()))
	if _, err := buf.Write(timeBytes); err != nil {
		return "", fmt.Errorf("failed to write creation time: %w", err)
	}

	// 3. 写入算法类型
	if err := buf.WriteByte(byte(pubKey.PubKeyAlgo)); err != nil {
		return "", fmt.Errorf("failed to write algorithm: %w", err)
	}

	// 4. 写入公钥数据
	switch pubKey.PubKeyAlgo {
	case packet.PubKeyAlgoEdDSA:
		// ED25519 公钥长度固定为 32 字节
		if err := buf.WriteByte(32); err != nil {
			return "", fmt.Errorf("failed to write key length: %w", err)
		}
		if _, err := buf.Write(pubKey.PublicKey.(ed25519.PublicKey)); err != nil {
			return "", fmt.Errorf("failed to write public key: %w", err)
		}
	// 可以添加其他算法的支持
	default:
		return "", fmt.Errorf("unsupported public key algorithm: %v", pubKey.PubKeyAlgo)
	}

	// 5. 计算 SHA256 哈希
	hash := sha256.New()
	hash.Write(buf.Bytes())
	fingerprint := hash.Sum(nil)

	// 6. 转换为十六进制字符串
	return fmt.Sprintf("%X", fingerprint), nil
}

// 用于验证指纹的辅助函数
func VerifyFingerprint(entity *openpgp.Entity, expectedFingerprint string) bool {
	actualFingerprint, err := CalculateFingerprint(entity.PrimaryKey)
	if err != nil {
		return false
	}
	return actualFingerprint == expectedFingerprint
}

// 获取指纹的最后16个字符
func GetLastSixteen(fingerprint string) string {
	if len(fingerprint) < 16 {
		return fingerprint
	}
	return fingerprint[len(fingerprint)-16:]
}
