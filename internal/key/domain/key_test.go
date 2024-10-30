package domain

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"gpgenie/internal/config"
	"gpgenie/internal/logger"
	"gpgenie/models"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewEntity(t *testing.T) {
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
			name:    "empty config",
			cfg:     config.KeyGenerationConfig{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity, err := NewEntity(tt.cfg)
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

func TestSerializeKeys(t *testing.T) {
	cfg := config.KeyGenerationConfig{
		Name:    "Test User",
		Email:   "test@example.com",
		Comment: "Test Key",
	}

	tests := []struct {
		name        string
		setupEntity func() (*openpgp.Entity, error)
		encryptor   *MockEncryptor
		wantErr     bool
	}{
		{
			name: "valid entity",
			setupEntity: func() (*openpgp.Entity, error) {
				return NewEntity(cfg)
			},
			encryptor: func() *MockEncryptor {
				m := new(MockEncryptor)
				m.On("Encrypt", mock.Anything).Return("encrypted-key", nil)
				return m
			}(),
			wantErr: false,
		},
		{
			name: "encryption error",
			setupEntity: func() (*openpgp.Entity, error) {
				return NewEntity(cfg)
			},
			encryptor: func() *MockEncryptor {
				m := new(MockEncryptor)
				m.On("Encrypt", mock.Anything).Return("", assert.AnError)
				return m
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity, err := tt.setupEntity()
			if err != nil {
				t.Fatal(err)
			}

			pubKey, privKey, err := SerializeKeys(entity, tt.encryptor)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, pubKey)
				assert.Empty(t, privKey)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, pubKey)
				assert.NotEmpty(t, privKey)
			}
		})
	}
}

func TestDisplayKeys(t *testing.T) {
	keys := []models.KeyInfo{
		{
			Fingerprint:        "ABC123",
			Score:              100,
			UniqueLettersCount: 5,
		},
		{
			Fingerprint:        "DEF456",
			Score:              200,
			UniqueLettersCount: 6,
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	DisplayKeys(keys)

	w.Close()
	os.Stdout = oldStdout

	var output []byte
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, string(output), "ABC123")
	assert.Contains(t, string(output), "DEF456")
}

func TestExportKey(t *testing.T) {
	tempDir := t.TempDir()
	log, err := logger.InitLogger(&config.LoggingConfig{})
	if err != nil {
		t.Fatal(err)
	}

	key := &models.KeyInfo{
		Fingerprint: "TEST123",
		PublicKey:   "public-key-data",
		PrivateKey:  "private-key-data",
	}

	tests := []struct {
		name        string
		outputDir   string
		exportArmor bool
		setupMock   func() *MockEncryptor
		wantErr     bool
	}{
		{
			name:        "export with armor",
			outputDir:   tempDir,
			exportArmor: true,
			setupMock: func() *MockEncryptor {
				m := new(MockEncryptor)
				return m
			},
			wantErr: false,
		},
		{
			name:        "export without armor",
			outputDir:   tempDir,
			exportArmor: false,
			setupMock: func() *MockEncryptor {
				m := new(MockEncryptor)
				m.On("Encrypt", "private-key-data").Return("encrypted-private-key", nil)
				return m
			},
			wantErr: false,
		},
		{
			name:        "invalid output directory",
			outputDir:   "/invalid/path",
			exportArmor: true,
			setupMock: func() *MockEncryptor {
				return new(MockEncryptor)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockEncryptor := tt.setupMock()
			err := ExportKey(key, tt.outputDir, tt.exportArmor, mockEncryptor, log)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify files exist
				pubKeyPath := filepath.Join(tt.outputDir, key.Fingerprint+"_pub.key")
				privKeyPath := filepath.Join(tt.outputDir, key.Fingerprint+"_priv.key")

				assert.FileExists(t, pubKeyPath)
				assert.FileExists(t, privKeyPath)
			}
		})
	}
}

// Benchmark tests
func BenchmarkNewEntity(b *testing.B) {
	cfg := config.KeyGenerationConfig{
		Name:    "Test User",
		Email:   "test@example.com",
		Comment: "Test Key",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entity, err := NewEntity(cfg)
		if err != nil {
			b.Fatal(err)
		}
		if entity == nil {
			b.Fatal("entity is nil")
		}
	}
}

func BenchmarkSerializeKeys(b *testing.B) {
	cfg := config.KeyGenerationConfig{
		Name:    "Test User",
		Email:   "test@example.com",
		Comment: "Test Key",
	}

	entity, err := NewEntity(cfg)
	require.NoError(b, err)

	mockEncryptor := new(MockEncryptor)
	mockEncryptor.On("Encrypt", mock.Anything).Return("encrypted-key", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pubKey, privKey, err := SerializeKeys(entity, mockEncryptor)
		if err != nil {
			b.Fatal(err)
		}
		if pubKey == "" || privKey == "" {
			b.Fatal("empty keys")
		}
	}
}

func BenchmarkExportKey(b *testing.B) {
	tempDir := b.TempDir()
	log, err := logger.InitLogger(&config.LoggingConfig{})
	if err != nil {
		b.Fatal(err)
	}

	key := &models.KeyInfo{
		Fingerprint: "TEST123",
		PublicKey:   "public-key-data",
		PrivateKey:  "private-key-data",
	}

	mockEncryptor := new(MockEncryptor)
	mockEncryptor.On("Encrypt", mock.Anything).Return("encrypted-key", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ExportKey(key, tempDir, false, mockEncryptor, log)
		if err != nil {
			b.Fatal(err)
		}
	}
}
