package database

import (
	"fmt"
	"time"

	"gpgenie/internal/config"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
}

func Connect(cfg config.DatabaseConfig) (*DB, error) {
	var dialector gorm.Dialector

	switch cfg.Type {
	case "postgres":
		dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName)
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(cfg.DBName)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Type)
	}

	// 配置 GORM 日志
	var gormLogger logger.Interface
	switch cfg.LogLevel {
	case "debug":
		gormLogger = logger.Default.LogMode(logger.Info)
	case "info":
		gormLogger = logger.Default.LogMode(logger.Warn)
	case "warn":
		gormLogger = logger.Default.LogMode(logger.Error)
	default:
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	// 初始化 GORM DB
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger:      gormLogger,
		QueryFields: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database with GORM: %w", err)
	}

	// 获取 sql.DB 对象用于低级操作
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get generic database object: %w", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// 验证连接
	if err = sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	if db != nil {
		sqlDB, err := db.DB.DB()
		if err != nil {
			return fmt.Errorf("failed to get generic database object: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}
