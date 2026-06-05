package redis

import (
	"context"
	"dpsystem/configs"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func RedisInit(cfg *configs.RedisConfig, logger *zap.SugaredLogger) *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       0,
	})

	_, rErr := redisClient.Ping(context.Background()).Result()
	if rErr != nil {
		logger.Warn("redis init fail", "error", rErr)
		return nil
	}

	return redisClient
}
