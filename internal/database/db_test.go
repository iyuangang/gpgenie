package database

import (
	"testing"

	"gpgenie/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestConnect(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.DatabaseConfig
		wantErr bool
	}{
		{
			name: "sqlite memory connection",
			cfg: config.DatabaseConfig{
				Type:   "sqlite",
				DBName: ":memory:",
			},
			wantErr: false,
		},
		{
			name: "invalid database type",
			cfg: config.DatabaseConfig{
				Type: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, err := Connect(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
				if db != nil {
					err = db.Close()
					assert.NoError(t, err)
				}
			}
		})
	}
}
