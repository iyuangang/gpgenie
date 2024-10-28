package app

import (
	"fmt"

	"gpgenie/internal/config"
	"gpgenie/internal/database"
	"gpgenie/internal/key/service"
	"gpgenie/internal/logger"
	"gpgenie/internal/repository"
)

type App struct {
	Config     *config.Config
	DB         *database.DB
	Logger     *logger.Logger
	KeyService service.KeyService
	Repository repository.KeyRepository
}

// NewApp 初始化应用程序，通过依赖注入传入 Encryptor
func NewApp(configPath string) (*App, error) {
	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化日志
	log, err := logger.InitLogger(&cfg.Logging)
	if err != nil {
		return nil, fmt.Errorf("初始化日志失败: %w", err)
	}

	// 连接数据库
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Errorf("数据库连接错误: %v", err)
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 初始化仓储
	repo := repository.NewKeyRepository(db.DB)

	// 初始化 KeyService，并注入 Encryptor
	keyService := service.NewKeyService(repo, cfg.KeyGeneration, nil, log)

	return &App{
		Config:     cfg,
		DB:         db,
		Logger:     log,
		KeyService: keyService,
		Repository: repo,
	}, nil
}

func (a *App) Close() error {
	return a.DB.Close()
}
