package domain

import (
	"crypto/ed25519"
	"testing"
	"time"

	"gpgenie/internal/config"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockEncryptor struct {
	mock.Mock
}

func (m *MockEncryptor) Encrypt(data string) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}

func TestGenerateKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.KeyGenerationConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: config.KeyGenerationConfig{
				Name:    "Test User",
				Email:   "test@example.com",
				Comment: "Test Key",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			cfg: config.KeyGenerationConfig{
				Email:   "test@example.com",
				Comment: "Test Key",
			},
			wantErr: false,
		},
		{
			name: "invalid email",
			cfg: config.KeyGenerationConfig{
				Name:    "Test User",
				Email:   "invalid-email",
				Comment: "Test Key",
			},
			wantErr: false,
		},
		{
			name:    "empty config",
			cfg:     config.KeyGenerationConfig{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEncryptor := new(MockEncryptor)
			mockEncryptor.On("Encrypt", mock.Anything).Return("encrypted", nil)

			entity, err := GenerateKeyPair(tt.cfg, mockEncryptor)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, entity)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, entity)
				if tt.cfg.Name != "" {
					assert.Equal(t, tt.cfg.Name, entity.PrimaryIdentity().UserId.Name)
				}
			}

			mockEncryptor.AssertExpectations(t)
		})
	}
}

func TestGenerateBareKeyPair(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "successful generation",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, priv, err := GenerateBareKeyPair()

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, pub)
				assert.Nil(t, priv)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, pub)
				assert.NotNil(t, priv)

				// 验证密钥长度
				assert.Equal(t, ed25519.PublicKeySize, len(pub))
				assert.Equal(t, ed25519.PrivateKeySize, len(priv))

				// 验证密钥对的有效性
				message := []byte("test message")
				signature := ed25519.Sign(priv, message)
				assert.True(t, ed25519.Verify(pub, message, signature))
			}
		})
	}
}

func TestPackPrivateKey(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() (ed25519.PublicKey, ed25519.PrivateKey)
		wantErr bool
	}{
		{
			name: "valid keys",
			setup: func() (ed25519.PublicKey, ed25519.PrivateKey) {
				pub, priv, _ := GenerateBareKeyPair()
				return pub, priv
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pub, priv := tt.setup()
			entity, err := PackPrivateKey(pub, priv)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, entity)
			} else {
				require.NoError(t, err)
				require.NotNil(t, entity)

				// 验证实体属性
				assert.NotNil(t, entity.PrimaryKey)
				assert.NotNil(t, entity.PrivateKey)
				assert.Equal(t, packet.PubKeyAlgoEdDSA, entity.PrimaryKey.PubKeyAlgo)
				assert.NotNil(t, entity.Identities)
				assert.Empty(t, entity.Identities)
				assert.NotNil(t, entity.Subkeys)
				assert.Empty(t, entity.Subkeys)

				// 验证时间戳
				assert.True(t, entity.PrimaryKey.CreationTime.Before(time.Now()))
				assert.True(t, entity.PrimaryKey.CreationTime.After(time.Now().Add(-time.Minute)))
			}
		})
	}
}

// 基准测试
func BenchmarkGenerateKeyPair(b *testing.B) {
	cfg := config.KeyGenerationConfig{
		Name:    "Test User",
		Email:   "test@example.com",
		Comment: "Test Key",
	}
	mockEncryptor := new(MockEncryptor)
	mockEncryptor.On("Encrypt", mock.Anything).Return("encrypted", nil)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			entity, err := GenerateKeyPair(cfg, mockEncryptor)
			if err != nil {
				b.Fatal(err)
			}
			if entity == nil {
				b.Fatal("entity is nil")
			}
		}
	})
}

func BenchmarkGenerateBareKeyPair(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			pub, priv, err := GenerateBareKeyPair()
			if err != nil {
				b.Fatal(err)
			}
			if pub == nil || priv == nil {
				b.Fatal("keys are nil")
			}
		}
	})
}

func BenchmarkPackPrivateKey(b *testing.B) {
	pub, priv, err := GenerateBareKeyPair()
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			entity, err := PackPrivateKey(pub, priv)
			if err != nil {
				b.Fatal(err)
			}
			if entity == nil {
				b.Fatal("entity is nil")
			}
		}
	})
}

// 完整流程基准测试
func BenchmarkFullKeyGeneration(b *testing.B) {
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			pub, priv, err := GenerateBareKeyPair()
			if err != nil {
				b.Fatal(err)
			}
			entity, err := PackPrivateKey(pub, priv)
			if err != nil {
				b.Fatal(err)
			}
			if entity == nil {
				b.Fatal("entity is nil")
			}
		}
	})

	b.Run("Parallel", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				pub, priv, err := GenerateBareKeyPair()
				if err != nil {
					b.Fatal(err)
				}
				entity, err := PackPrivateKey(pub, priv)
				if err != nil {
					b.Fatal(err)
				}
				if entity == nil {
					b.Fatal("entity is nil")
				}
			}
		})
	})
}
