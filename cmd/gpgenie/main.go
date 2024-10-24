package main

import (
	"fmt"
	"os"
	"strings"

	"gpgenie/internal/config"
	"gpgenie/internal/database"
	"gpgenie/internal/key"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Initialize Viper and parse configuration
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize Logger and defer syncing
	logger.InitLogger(cfg)
	defer logger.SyncLogger()

	// Connect to the database
	db, err := database.Connect(*cfg)
	if err != nil {
		logger.Logger.Errorf("Database connection error: %v", err)
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer func() {
		if err := database.CloseDB(db); err != nil {
			logger.Logger.Errorf("Failed to close database: %v", err)
		}
	}()

	logger.Logger.Infof("Connected to database: %s", cfg.Database.DBName)

	// Initialize Repository
	repo := repository.NewKeyRepository(db)

	// Initialize Scorer
	scorer, err := key.NewScorer(repo, cfg)
	if err != nil {
		logger.Logger.Errorf("Scorer initialization error: %v", err)
		return fmt.Errorf("failed to initialize scorer: %w", err)
	}

	// Handle commands using Viper configuration
	if err := handleCommands(scorer, repo); err != nil {
		logger.Logger.Errorf("Command handling error: %v", err)
		return err
	}

	return nil
}

func loadConfig() (*config.Config, error) {
	// Define command-line flags using pflag
	pflag.String("config", "config/config.json", "Path to config file")
	pflag.Bool("generate-keys", false, "Generate GPG keys")
	pflag.Int("show-top", 0, "Show top N keys by score")
	pflag.Int("show-low-letter", 0, "Show N keys with lowest letter count")
	pflag.String("export-by-fingerprint", "", "Export key by last 16 characters of fingerprint")
	pflag.Bool("armor", false, "Export the key in ASCII Armor format without decoding")
	pflag.String("output-dir", ".", "Output directory for exported keys")
	pflag.Bool("analysis", false, "Analyze stored key data")
	pflag.Parse()

	// Initialize Viper
	viper.SetConfigFile(pflag.Lookup("config").Value.String())
	viper.SetConfigType("json")

	// Set environment variables prefix
	viper.SetEnvPrefix("GPGENIE")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	// Bind command-line flags to Viper
	if err := viper.BindPFlags(pflag.CommandLine); err != nil {
		return nil, fmt.Errorf("error binding flags: %w", err)
	}

	// Unmarshal into Config struct
	var cfg config.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unable to decode config into struct: %w", err)
	}

	return &cfg, nil
}

func handleCommands(scorer *key.Scorer, repo repository.KeyRepository) error {
	// Retrieve flag values from Viper
	generateKeys := viper.GetBool("generate-keys")
	showTopKeys := viper.GetInt("show-top")
	showLowLetterCount := viper.GetInt("show-low-letter")
	exportByFingerprint := viper.GetString("export-by-fingerprint")
	exportArmor := viper.GetBool("armor")
	outputDir := viper.GetString("output-dir")
	analysis := viper.GetBool("analysis")

	switch {
	case generateKeys:
		logger.Logger.Info("Starting key generation...")
		return scorer.GenerateKeys()
	case showTopKeys > 0:
		logger.Logger.Infof("Displaying top %d keys by score...", showTopKeys)
		return scorer.ShowTopKeys(showTopKeys)
	case showLowLetterCount > 0:
		logger.Logger.Infof("Displaying %d keys with lowest letter count...", showLowLetterCount)
		return scorer.ShowLowLetterCountKeys(showLowLetterCount)
	case exportByFingerprint != "":
		logger.Logger.Infof("Exporting key with fingerprint ending %s...", exportByFingerprint)
		return scorer.ExportKeyByFingerprint(exportByFingerprint, outputDir, exportArmor)
	case analysis:
		logger.Logger.Info("Starting data analysis...")
		return analyzeData(repo)
	default:
		pflag.Usage()
		return nil
	}
}

func analyzeData(repo repository.KeyRepository) error {
	analyzer := key.NewAnalyzer(repo)
	return analyzer.PerformAnalysis()
}
