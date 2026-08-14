// internal/key/domain/encryptor.go
package domain

// Encryptor 定义了加密操作的接口
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
}

// CloneableEncryptor can create an independent encryptor for a worker. This
// avoids serializing otherwise independent encryption operations behind a lock.
type CloneableEncryptor interface {
	Encryptor
	Clone() (Encryptor, error)
}
