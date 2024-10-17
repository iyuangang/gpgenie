package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"gpgenie/internal/config"
	"gpgenie/internal/database"
	"gpgenie/internal/key"
	"gpgenie/internal/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	logger.InitLogger()
	defer logger.Logger.Sync()

	db, err := database.Connect(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.CloseDB(db)

	sqlDB, err := db.DB()
	if err != nil {
		logger.Logger.Fatalf("Failed to get database instance: %v", err)
	}
	defer sqlDB.Close()
	setupGracefulShutdown(sqlDB)

	var encryptor *key.Encryptor
	if cfg.KeyEncryption.PublicKeyPath != "" {
		encryptor, err = key.NewEncryptor(&cfg.KeyEncryption)
		if err != nil {
			logger.Logger.Fatalf("Failed to load encryption public key: %v", err)
		}
		logger.Logger.Info("Encryption public key loaded successfully")
	}

	scorer := key.NewScorer(sqlDB, cfg, encryptor)

	if err := handleCommands(scorer); err != nil {
		return err
	}

	return nil
}

func loadConfig() (*config.Config, error) {
	configPath := flag.String("config", "config/config.json", "Path to config file")

	cfg, err := config.Load(*configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}

func handleCommands(scorer *key.Scorer) error {
	generateKeys := flag.Bool("generate-keys", false, "Generate GPG keys")
	showTopKeys := flag.Int("show-top", 0, "Show top N keys by score")
	showLowLetterCount := flag.Int("show-low-letter", 0, "Show N keys with lowest letter count")
	exportByFingerprint := flag.String("export-by-fingerprint", "", "Export key by last 16 characters of fingerprint")
	outputDir := flag.String("output-dir", ".", "Output directory for exported keys")

	flag.Parse()

	switch {
	case *generateKeys:
		return generateKeysCommand(scorer)
	case *showTopKeys > 0:
		return showTopKeysCommand(scorer, *showTopKeys)
	case *showLowLetterCount > 0:
		return showLowLetterCountKeysCommand(scorer, *showLowLetterCount)
	case *exportByFingerprint != "":
		return exportKeyByFingerprintCommand(scorer, *exportByFingerprint, *outputDir)
	default:
		flag.Usage()
		return nil
	}
}

func generateKeysCommand(scorer *key.Scorer) error {
	logger.Logger.Info("Starting key generation")
	if err := scorer.GenerateKeys(); err != nil {
		return fmt.Errorf("failed to generate keys: %w", err)
	}
	logger.Logger.Info("Key generation completed")
	return nil
}

func showTopKeysCommand(scorer *key.Scorer, n int) error {
	logger.Logger.Infof("Showing top %d keys", n)
	if err := scorer.ShowTopKeys(n); err != nil {
		return fmt.Errorf("failed to show top keys: %w", err)
	}
	return nil
}

func showLowLetterCountKeysCommand(scorer *key.Scorer, n int) error {
	logger.Logger.Infof("Showing %d keys with lowest letter count", n)
	if err := scorer.ShowLowLetterCountKeys(n); err != nil {
		return fmt.Errorf("failed to show keys with lowest letter count: %w", err)
	}
	return nil
}

func exportKeyByFingerprintCommand(scorer *key.Scorer, fingerprint, outputDir string) error {
	logger.Logger.Infof("Exporting key with fingerprint ending in %s", fingerprint)
	if err := scorer.ExportKeyByFingerprint(fingerprint, outputDir); err != nil {
		return fmt.Errorf("failed to export key by fingerprint: %w", err)
	}
	logger.Logger.Infof("Successfully exported key to %s/%s.enc", outputDir, fingerprint)
	return nil
}

func setupGracefulShutdown(db *sql.DB) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logger.Logger.Info("Received interrupt signal, shutting down...")
		if err := db.Close(); err != nil {
			logger.Logger.Errorf("Error closing database connection: %v", err)
		}
		os.Exit(0)
	}()
}
