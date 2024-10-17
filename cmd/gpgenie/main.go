package main

import (
	"flag"
	"fmt"
	"os"

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
	// Initialize logger first to capture all logs
	logger.InitLogger()
	defer logger.SyncLogger()

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to the database
	db, err := database.Connect(cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer database.CloseDB(db)

	// Initialize Scorer
	scorer, err := key.NewScorer(db, cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize scorer: %w", err)
	}

	// Handle command-line commands
	if err := handleCommands(scorer); err != nil {
		return err
	}

	return nil
}

func loadConfig() (*config.Config, error) {
	configPath := flag.String("config", "config/config.json", "Path to config file")

	return config.Load(*configPath)
}

func handleCommands(scorer *key.Scorer) error {
	// Define command-line flags
	generateKeys := flag.Bool("generate-keys", false, "Generate GPG keys")
	showTopKeys := flag.Int("show-top", 0, "Show top N keys by score")
	showLowLetterCount := flag.Int("show-low-letter", 0, "Show N keys with lowest letter count")
	exportByFingerprint := flag.String("export-by-fingerprint", "", "Export key by last 16 characters of fingerprint")
	outputDir := flag.String("output-dir", ".", "Output directory for exported keys")

	// Re-parse flags to include new ones
	flag.Parse()

	switch {
	case *generateKeys:
		return scorer.GenerateKeys()
	case *showTopKeys > 0:
		return scorer.ShowTopKeys(*showTopKeys)
	case *showLowLetterCount > 0:
		return scorer.ShowLowLetterCountKeys(*showLowLetterCount)
	case *exportByFingerprint != "":
		return scorer.ExportKeyByFingerprint(*exportByFingerprint, *outputDir)
	default:
		flag.Usage()
		return nil
	}
}
