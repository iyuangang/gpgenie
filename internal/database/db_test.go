package database

import (
	"testing"

	"gpgenie/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConnectSQLite(t *testing.T) {
	cfg := config.DatabaseConfig{
		Type:            "sqlite",
		DBName:          "test.db",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30,
		LogLevel:        "silent",
	}

	db, err := Connect(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.Close()
	assert.NoError(t, err)
}
