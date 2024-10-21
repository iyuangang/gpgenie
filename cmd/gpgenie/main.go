package main

import (
	"flag"
	"fmt"
	"os"

	"gpgenie/internal/config"
	"gpgenie/internal/database"
	"gpgenie/internal/key"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 加载配置
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 初始化 Logger 并传递配置
	logger.InitLogger(cfg)
	defer logger.SyncLogger()
	// 连接数据库
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

	// 初始化 Repository
	repo := repository.NewKeyRepository(db)

	// 初始化 Scorer
	scorer, err := key.NewScorer(repo, cfg)
	if err != nil {
		logger.Logger.Errorf("Scorer initialization error: %v", err)
		return fmt.Errorf("failed to initialize scorer: %w", err)
	}

	// 处理命令行命令
	if err := handleCommands(scorer); err != nil {
		logger.Logger.Errorf("Command handling error: %v", err)
		return err
	}

	return nil
}

func loadConfig() (*config.Config, error) {
	configPath := flag.String("config", "config/config.json", "Path to config file")
	return config.Load(*configPath)
}

func handleCommands(scorer *key.Scorer) error {
	// 定义命令行标志
	generateKeys := flag.Bool("generate-keys", false, "Generate GPG keys")
	showTopKeys := flag.Int("show-top", 0, "Show top N keys by score")
	showLowLetterCount := flag.Int("show-low-letter", 0, "Show N keys with lowest letter count")
	exportByFingerprint := flag.String("export-by-fingerprint", "", "Export key by last 16 characters of fingerprint")
	outputDir := flag.String("output-dir", ".", "Output directory for exported keys")

	// 重新解析标志以包含新的标志
	flag.Parse()

	switch {
	case *generateKeys:
		logger.Logger.Info("Starting key generation...")
		return scorer.GenerateKeys()
	case *showTopKeys > 0:
		logger.Logger.Infof("Displaying top %d keys by score...", *showTopKeys)
		return scorer.ShowTopKeys(*showTopKeys)
	case *showLowLetterCount > 0:
		logger.Logger.Infof("Displaying %d keys with lowest letter count...", *showLowLetterCount)
		return scorer.ShowLowLetterCountKeys(*showLowLetterCount)
	case *exportByFingerprint != "":
		logger.Logger.Infof("Exporting key with fingerprint ending %s...", *exportByFingerprint)
		return scorer.ExportKeyByFingerprint(*exportByFingerprint, *outputDir)
	default:
		flag.Usage()
		return nil
	}
}
