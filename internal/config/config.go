package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Environment   string              `mapstructure:"environment"`
	Database      DatabaseConfig      `mapstructure:"database"`
	KeyGeneration KeyGenerationConfig `mapstructure:"key_generation"`
	Vanity        VanityConfig        `mapstructure:"vanity"`
	Logging       LoggingConfig       `mapstructure:"logging"`
}

type VanityConfig struct {
	MinRun         int    `mapstructure:"min_run"`
	SaveToDatabase bool   `mapstructure:"save_to_database"`
	Backend        string `mapstructure:"backend"`
	OpenCLDevices  string `mapstructure:"opencl_devices"`
	GPUKeyBatch    int    `mapstructure:"gpu_key_batch"`
	GPUWorkItems   uint64 `mapstructure:"gpu_work_items"`
}

func (c VanityConfig) Validate() error {
	if c.MinRun != 0 && (c.MinRun < 1 || c.MinRun > 16) {
		return fmt.Errorf("vanity.min_run must be between 1 and 16")
	}
	switch c.Backend {
	case "", "cpu", "opencl", "hybrid", "auto":
	default:
		return fmt.Errorf("vanity.backend must be cpu, opencl, hybrid, or auto")
	}
	if c.GPUKeyBatch < 0 {
		return fmt.Errorf("vanity.gpu_key_batch must not be negative")
	}
	if c.GPUKeyBatch > 65536 {
		return fmt.Errorf("vanity.gpu_key_batch must not exceed 65536")
	}
	if c.GPUWorkItems > (1<<27)-1 {
		return fmt.Errorf("vanity.gpu_work_items must not exceed 134217727")
	}
	return nil
}

type LoggingConfig struct {
	LogLevel string `mapstructure:"log_level"`
	LogFile  string `mapstructure:"log_file"`
}

type DatabaseConfig struct {
	Type            string `mapstructure:"type"`
	Host            string `mapstructure:"host"`
	Port            int    `mapstructure:"port"`
	User            string `mapstructure:"user"`
	Password        string `mapstructure:"password"`
	DBName          string `mapstructure:"dbname"`
	MaxOpenConns    int    `mapstructure:"max_open_conns"`
	MaxIdleConns    int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetime int    `mapstructure:"conn_max_lifetime"`
	LogLevel        string `mapstructure:"log_level"`
}

type KeyGenerationConfig struct {
	NumGeneratorWorkers int    `mapstructure:"num_generator_workers"`
	NumScorerWorkers    int    `mapstructure:"num_scorer_workers"`
	TotalKeys           int    `mapstructure:"total_keys"`
	MinScore            int    `mapstructure:"min_score"`
	MaxLettersCount     int    `mapstructure:"max_letters_count"`
	BatchSize           int    `mapstructure:"batch_size"`
	Name                string `mapstructure:"name"`
	Comment             string `mapstructure:"comment"`
	Email               string `mapstructure:"email"`
	EncryptorPublicKey  string `mapstructure:"encryptor_public_key"`
}

func (c KeyGenerationConfig) Validate() error {
	switch {
	case c.NumGeneratorWorkers <= 0:
		return fmt.Errorf("num_generator_workers must be greater than zero")
	case c.NumScorerWorkers <= 0:
		return fmt.Errorf("num_scorer_workers must be greater than zero")
	case c.TotalKeys < 0:
		return fmt.Errorf("total_keys must not be negative")
	case c.BatchSize <= 0:
		return fmt.Errorf("batch_size must be greater than zero")
	case c.MaxLettersCount < 0 || c.MaxLettersCount > 16:
		return fmt.Errorf("max_letters_count must be between 0 and 16")
	}
	return nil
}

func Load(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("json")

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	// 绑定环境变量
	v.SetEnvPrefix("GPGENIE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	for _, key := range []string{
		"environment",
		"database.type", "database.host", "database.port", "database.user",
		"database.password", "database.dbname", "database.max_open_conns",
		"database.max_idle_conns", "database.conn_max_lifetime", "database.log_level",
		"key_generation.num_generator_workers", "key_generation.num_scorer_workers",
		"key_generation.total_keys", "key_generation.min_score",
		"key_generation.max_letters_count", "key_generation.batch_size",
		"key_generation.name", "key_generation.comment", "key_generation.email",
		"key_generation.encryptor_public_key",
		"vanity.min_run", "vanity.save_to_database", "vanity.backend",
		"vanity.opencl_devices", "vanity.gpu_key_batch", "vanity.gpu_work_items",
		"logging.log_level", "logging.log_file",
	} {
		if err := v.BindEnv(key); err != nil {
			return nil, fmt.Errorf("bind environment variable %s: %w", key, err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	if err := cfg.Vanity.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
