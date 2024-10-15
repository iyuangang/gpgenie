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

// Connect 使用 GORM 连接数据库
func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	var err error

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

	// 配置 GORM 日志级别
	newLogger := logger.Default.LogMode(logger.Info)

	// 初始化 GORM DB
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database with GORM: %w", err)
	}

	// 获取通用数据库对象 sql.DB，以便进行底层操作
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

	return db, nil
}

// CloseDB 关闭 GORM 数据库连接
func CloseDB(db *gorm.DB) error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return fmt.Errorf("failed to get generic database object: %w", err)
		}
		return sqlDB.Close()
	}
	return nil
}
