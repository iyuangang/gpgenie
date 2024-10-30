package domain

import (
	"testing"

	"gpgenie/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEncryptor := new(MockEncryptor)
			entity, err := GenerateKeyPair(tt.cfg, mockEncryptor)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, entity)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, entity)
				assert.Equal(t, tt.cfg.Name, entity.PrimaryIdentity().UserId.Name)
			}
		})
	}
}

func BenchmarkGenerateKeyPair(b *testing.B) {
	cfg := config.KeyGenerationConfig{
		Name:    "Test User",
		Email:   "test@example.com",
		Comment: "Test Key",
	}
	mockEncryptor := new(MockEncryptor)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entity, err := GenerateKeyPair(cfg, mockEncryptor)
		if err != nil {
			b.Fatal(err)
		}
		if entity == nil {
			b.Fatal("entity is nil")
		}
	}
}
