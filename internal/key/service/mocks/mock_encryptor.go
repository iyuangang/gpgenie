package mocks

// MockEncryptor 是 Encryptor 接口的模拟实现
type MockEncryptor struct {
	EncryptFunc func(plaintext string) (string, error)
}

func (m *MockEncryptor) Encrypt(plaintext string) (string, error) {
	if m.EncryptFunc != nil {
		return m.EncryptFunc(plaintext)
	}
	return "", nil
}
