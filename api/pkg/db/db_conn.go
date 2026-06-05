package db

import (
	"dpsystem/configs"
	"dpsystem/internal/repositories"
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func DatabaseInit(cfg *configs.DatabaseConfig, logger *zap.SugaredLogger) *gorm.DB {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.Name)

	var db *gorm.DB
	var err error
	maxRetries := 10

	for i := 0; i < maxRetries; i++ {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			migrateErr := db.AutoMigrate(
				repositories.MediaTaskModel{},
			)
			if migrateErr != nil {
				logger.Errorw("failed to migrate db", "error", err)
			}
			return db
		}
		logger.Warnf("failed to connect to database (attempt %d/%d): %v", i+1, maxRetries, err)
		time.Sleep(2 * time.Second)
	}

	logger.Fatalf("failed to connect to database after %d attempts: %v", maxRetries, err)
	return nil
}
