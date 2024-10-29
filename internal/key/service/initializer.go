// internal/key/service/service_initializer.go
package service

import (
	"gpgenie/internal/config"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"
)

// InitializeKeyService initializes the KeyService with all dependencies.
func InitializeKeyService(cfg config.Config, repo repository.KeyRepository, log *logger.Logger) (KeyService, error) {
	// Initialize Encryptor
	encryptor, err := NewPGPEncryptor(cfg.KeyGeneration.EncryptorPublicKey)
	if err != nil {
		return nil, err
	}

	// Create KeyService
	keyService := NewKeyService(repo, cfg.KeyGeneration, encryptor, log)
	return keyService, nil
}
