// internal/key/domain/encryptor.go
package domain

// Encryptor 定义了加密操作的接口
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
}
