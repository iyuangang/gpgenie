package app

import (
	"fmt"

	"github.com/iyuangang/gpgenie/internal/config"
	"github.com/iyuangang/gpgenie/internal/database"
	"github.com/iyuangang/gpgenie/internal/key/service"
	"github.com/iyuangang/gpgenie/internal/logger"
	"github.com/iyuangang/gpgenie/internal/repository"
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
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 初始化日志
	log, err := logger.InitLogger(&cfg.Logging)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	// 连接数据库
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Errorf("failed to connect to database: %v", err)
		log.SyncLogger()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// 初始化仓储
	repo := repository.NewKeyRepository(db.DB)

	// 初始化 KeyService，并注入 Encryptor
	keyService, err := service.InitializeKeyService(cfg, repo, log)
	if err != nil {
		_ = db.Close()
		log.SyncLogger()
		return nil, fmt.Errorf("failed to initialize KeyService: %w", err)
	}

	return &App{
		Config:     cfg,
		DB:         db,
		Logger:     log,
		KeyService: keyService,
		Repository: repo,
	}, nil
}

func (a *App) Close() error {
	if a.Logger != nil {
		a.Logger.SyncLogger()
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
