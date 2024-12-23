package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Environment   string              `mapstructure:"environment"`
	Database      DatabaseConfig      `mapstructure:"database"`
	KeyGeneration KeyGenerationConfig `mapstructure:"key_generation"`
	Logging       LoggingConfig       `mapstructure:"logging"`
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

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("json")

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	// 绑定环境变量
	viper.SetEnvPrefix("GPGENIE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
