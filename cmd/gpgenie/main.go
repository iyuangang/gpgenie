package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"gpgenie/internal/config"
	"gpgenie/internal/database"
	"gpgenie/internal/key"
	"gpgenie/internal/logger"
)

func main() {
	configPath := flag.String("config", "config/config.json", "Path to config file")
	generateKeys := flag.Bool("generate-keys", false, "Generate GPG keys")
	showTopKeys := flag.Int("show-top", 0, "Show top N keys by score")
	showLowLetterCount := flag.Int("show-low-letter", 0, "Show N keys with lowest letter count")
	exportByFingerprint := flag.String("export-by-fingerprint", "", "Export key by last 16 characters of fingerprint")
	outputDir := flag.String("output-dir", ".", "Output directory for exported keys")
	flag.Parse()

	logger.InitLogger()
	defer logger.Logger.Sync()

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Logger.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		logger.Logger.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.CloseDB(db)
	
	sqlDB, err := db.DB()
	if err != nil {
		logger.Logger.Fatalf("Failed to get database instance: %v", err)
	}
	defer sqlDB.Close()

	// Load encryption public key if provided
	var encryptor *key.Encryptor
	if cfg.KeyEncryption.PublicKeyPath != "" {
		encryptor, err = key.NewEncryptor(&cfg.KeyEncryption)
		if err != nil {
			logger.Logger.Fatalf("Failed to load encryption public key: %v", err)
		}
		logger.Logger.Info("Encryption public key loaded successfully")
	}

	s := key.New(sqlDB, cfg, encryptor)

	if *generateKeys {
		err = s.GenerateKeys()
		if err != nil {
			logger.Logger.Fatalf("Failed to generate keys: %v", err)
		}
		return
	}

	if *showTopKeys > 0 {
		err = s.ShowTopKeys(*showTopKeys)
		if err != nil {
			logger.Logger.Fatalf("Failed to show top keys: %v", err)
		}
		return
	}

	if *showLowLetterCount > 0 {
		err = s.ShowLowLetterCountKeys(*showLowLetterCount)
		if err != nil {
			logger.Logger.Fatalf("Failed to show keys with lowest letter count: %v", err)
		}
		return
	}

	if *exportByFingerprint != "" {
		err = s.ExportKeyByFingerprint(*exportByFingerprint, *outputDir)
		if err != nil {
			logger.Logger.Fatalf("Failed to export key by fingerprint: %v", err)
		}
		logger.Logger.Infof("Successfully exported key to %s/%s.enc", *outputDir, *exportByFingerprint)
		return
	}

	logger.Logger.Info("Processing completed successfully")

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	logger.Logger.Info("Shutting down gracefully...")
	// Perform any cleanup here
	database.CloseDB(db)
}
