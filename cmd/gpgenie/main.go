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
	exportTopKeys := flag.Int("export-top", 0, "Export top N keys by score")
	exportLowLetterCount := flag.Int("export-low-letter", 0, "Export N keys with lowest letter count")
	outputFile := flag.String("output", "exported_keys.csv", "Output file for exported keys")
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
		logger.Logger.Info("Finished generating GPG keys")
		return
	}

	if *exportTopKeys > 0 {
		err = s.ExportTopKeys(*exportTopKeys, *outputFile)
		if err != nil {
			logger.Logger.Fatalf("Failed to export top keys: %v", err)
		}
		return
	}

	if *exportLowLetterCount > 0 {
		err = s.ExportLowLetterCountKeys(*exportLowLetterCount, *outputFile)
		if err != nil {
			logger.Logger.Fatalf("Failed to export low letter count keys: %v", err)
		}
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
