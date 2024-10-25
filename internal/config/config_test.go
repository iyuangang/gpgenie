package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	cfg, err := Load("config/config_test.json")
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "development", cfg.Environment)
	assert.Equal(t, "postgres", cfg.Database.Type)
	// 其他断言根据测试配置文件内容进行
}
