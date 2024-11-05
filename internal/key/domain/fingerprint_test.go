package domain

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateFingerprint(t *testing.T) {
	tests := []struct {
		name      string
		setupKey  func() *packet.PublicKey
		wantError bool
	}{
		{
			name: "valid ED25519 key",
			setupKey: func() *packet.PublicKey {
				pub, _, _ := ed25519.GenerateKey(rand.Reader)
				return &packet.PublicKey{
					CreationTime: time.Now(),
					PubKeyAlgo:   packet.PubKeyAlgoEdDSA,
					PublicKey:    pub,
				}
			},
			wantError: false,
		},
		{
			name: "unsupported algorithm",
			setupKey: func() *packet.PublicKey {
				return &packet.PublicKey{
					CreationTime: time.Now(),
					PubKeyAlgo:   packet.PubKeyAlgoRSA,
					PublicKey:    []byte("dummy"),
				}
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubKey := tt.setupKey()
			fingerprint, err := CalculateFingerprint(pubKey)

			if tt.wantError {
				assert.Error(t, err)
				assert.Empty(t, fingerprint)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, fingerprint)

				// 验证指纹格式
				_, err := hex.DecodeString(fingerprint)
				assert.NoError(t, err, "fingerprint should be valid hex string")

				// 验证指纹长度 (SHA256 = 32 bytes = 64 hex chars)
				assert.Equal(t, 64, len(fingerprint))
			}
		})
	}
}

func TestVerifyFingerprint(t *testing.T) {
	tests := []struct {
		name                string
		setupEntity         func() *openpgp.Entity
		expectedFingerprint string
		want                bool
	}{
		{
			name: "matching fingerprint",
			setupEntity: func() *openpgp.Entity {
				pub, priv, _ := GenerateBareKeyPair()
				entity, _ := PackPrivateKey(pub, priv)
				return entity
			},
			expectedFingerprint: "", // 将在运行时填充
			want:                true,
		},
		{
			name: "non-matching fingerprint",
			setupEntity: func() *openpgp.Entity {
				pub, priv, _ := GenerateBareKeyPair()
				entity, _ := PackPrivateKey(pub, priv)
				return entity
			},
			expectedFingerprint: "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
			want:                false,
		},
		{
			name: "invalid entity",
			setupEntity: func() *openpgp.Entity {
				return &openpgp.Entity{
					PrimaryKey: &packet.PublicKey{
						PubKeyAlgo: packet.PubKeyAlgoRSA, // 不支持的算法
					},
				}
			},
			expectedFingerprint: "dummy",
			want:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := tt.setupEntity()

			if tt.expectedFingerprint == "" {
				// 为匹配测试计算实际指纹
				actualFingerprint, err := CalculateFingerprint(entity.PrimaryKey)
				require.NoError(t, err)
				tt.expectedFingerprint = actualFingerprint
			}

			result := VerifyFingerprint(entity, tt.expectedFingerprint)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestGetLastSixteen(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint string
		want        string
	}{
		{
			name:        "full length fingerprint",
			fingerprint: "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF",
			want:        "0123456789ABCDEF",
		},
		{
			name:        "short fingerprint",
			fingerprint: "0123456789ABCDEF",
			want:        "0123456789ABCDEF",
		},
		{
			name:        "very short fingerprint",
			fingerprint: "0123",
			want:        "0123",
		},
		{
			name:        "empty fingerprint",
			fingerprint: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLastSixteen(tt.fingerprint)
			assert.Equal(t, tt.want, got)
		})
	}
}

// 基准测试
func BenchmarkCalculateFingerprint(b *testing.B) {
	pub, _, err := GenerateBareKeyPair()
	require.NoError(b, err)

	pubKey := &packet.PublicKey{
		CreationTime: time.Now(),
		PubKeyAlgo:   packet.PubKeyAlgoEdDSA,
		PublicKey:    pub,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fingerprint, err := CalculateFingerprint(pubKey)
			if err != nil {
				b.Fatal(err)
			}
			if len(fingerprint) != 64 {
				b.Fatal("invalid fingerprint length")
			}
		}
	})
}

func BenchmarkVerifyFingerprint(b *testing.B) {
	pub, priv, err := GenerateBareKeyPair()
	require.NoError(b, err)

	entity, err := PackPrivateKey(pub, priv)
	require.NoError(b, err)

	fingerprint, err := CalculateFingerprint(entity.PrimaryKey)
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if !VerifyFingerprint(entity, fingerprint) {
				b.Fatal("fingerprint verification failed")
			}
		}
	})
}

func BenchmarkGetLastSixteen(b *testing.B) {
	fingerprint := "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			result := GetLastSixteen(fingerprint)
			if len(result) != 16 {
				b.Fatal("invalid result length")
			}
		}
	})
}

// 完整流程基准测试
func BenchmarkFullFingerprintProcess(b *testing.B) {
	b.Run("Sequential", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// 生成密钥对
			pub, priv, err := GenerateBareKeyPair()
			if err != nil {
				b.Fatal(err)
			}

			// 打包成实体
			entity, err := PackPrivateKey(pub, priv)
			if err != nil {
				b.Fatal(err)
			}

			// 计算指纹
			fingerprint, err := CalculateFingerprint(entity.PrimaryKey)
			if err != nil {
				b.Fatal(err)
			}

			// 验证指纹
			if !VerifyFingerprint(entity, fingerprint) {
				b.Fatal("fingerprint verification failed")
			}

			// 获取最后16位
			lastSixteen := GetLastSixteen(fingerprint)
			if len(lastSixteen) != 16 {
				b.Fatal("invalid last sixteen length")
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

				fingerprint, err := CalculateFingerprint(entity.PrimaryKey)
				if err != nil {
					b.Fatal(err)
				}

				if !VerifyFingerprint(entity, fingerprint) {
					b.Fatal("fingerprint verification failed")
				}

				lastSixteen := GetLastSixteen(fingerprint)
				if len(lastSixteen) != 16 {
					b.Fatal("invalid last sixteen length")
				}
			}
		})
	})
}
