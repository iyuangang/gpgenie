package config

import (
	"github.com/spf13/viper"
)

type Config struct {
	Database       DatabaseConfig      `mapstructure:"database"`
	Processing     ProcessingConfig    `mapstructure:"processing"`
	KeyGeneration  KeyGenerationConfig `mapstructure:"key_generation"`
	KeyEncryption  KeyEncryptionConfig `mapstructure:"key_encryption"`
}

type DatabaseConfig struct {
	Type            string `mapstructure:"type"`             // "postgres" or "sqlite"
	Host            string `mapstructure:"host"`             // Only for postgres
	Port            int    `mapstructure:"port"`             // Only for postgres
	User            string `mapstructure:"user"`             // Only for postgres
	Password        string `mapstructure:"password"`         // Only for postgres
	DBName          string `mapstructure:"dbname"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"` // in seconds
}

type ProcessingConfig struct {
	BatchSize          int `mapstructure:"batch_size"`
	MaxConcurrentFiles int `mapstructure:"max_concurrent_files"`
}

type KeyGenerationConfig struct {
	TotalKeys       int    `mapstructure:"total_keys"`
	NumWorkers      int    `mapstructure:"num_workers"`
	MinScore        int    `mapstructure:"min_score"`
	MaxLettersCount int    `mapstructure:"max_letters_count"`
	Name            string `mapstructure:"name"`
	Comment         string `mapstructure:"comment"`
	Email           string `mapstructure:"email"`
}

type KeyEncryptionConfig struct {
	PublicKeyPath string `mapstructure:"public_key_path"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("json")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
