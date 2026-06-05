package configs

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Database *DatabaseConfig
	API      *APIConfig
	Storage  *StorageConfig
	Rabbit   *RabbitConfig
	Redis    *RedisConfig
}

type DatabaseConfig struct {
	Name string
	Host string
	Port string
	User string
	Pass string
}

type APIConfig struct {
	Port string
}

type StorageConfig struct {
	Endpoint                  string
	PublicEndpoint            string
	AccessKey                 string
	SecretKey                 string
	UploadsBucket             string
	ResultsBucket             string
	LinkValidityPeriodInHours string
}

type RabbitConfig struct {
	AmqpUrl       string
	MaxRetries    string
	RetryInterval string
}

type RedisConfig struct {
	Addr     string
	Password string
}

func LoadConfig() *Config {
	godotenv.Load()

	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")

	port := os.Getenv("API_PORT")

	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	minioAccessKey := os.Getenv("MINIO_USER")
	minioSecretKey := os.Getenv("MINIO_PASS")
	minioUploadsBucket := os.Getenv("MINIO_UPLOADS_BUCKET")
	minioResultsBucket := os.Getenv("MINIO_RESULTS_BUCKET")
	linkValidityPeriodInHours := os.Getenv("LINK_VALIDITY_PERIOD_IN_HOURS")

	minioPublicEndpoint := os.Getenv("MINIO_PUBLIC_ENDPOINT")
	if minioPublicEndpoint == "" {
		minioPublicEndpoint = minioEndpoint
	}

	amqpUrl := os.Getenv("AMQP_URL")
	maxRetries := os.Getenv("MAX_RETRIES")
	retryInterval := os.Getenv("RETRY_INTERVAL")

	redisAddr := os.Getenv("REDIS_ADDR")
	redisPass := os.Getenv("REDIS_PASSWORD")

	dbCfg := DatabaseConfig{
		Name: dbName,
		Host: dbHost,
		Port: dbPort,
		User: dbUser,
		Pass: dbPass,
	}

	apiCfg := APIConfig{
		Port: port,
	}

	storageCfg := StorageConfig{
		Endpoint:                  minioEndpoint,
		PublicEndpoint:            minioPublicEndpoint,
		AccessKey:                 minioAccessKey,
		SecretKey:                 minioSecretKey,
		UploadsBucket:             minioUploadsBucket,
		ResultsBucket:             minioResultsBucket,
		LinkValidityPeriodInHours: linkValidityPeriodInHours,
	}

	rabbitCfg := RabbitConfig{
		AmqpUrl:       amqpUrl,
		MaxRetries:    maxRetries,
		RetryInterval: retryInterval,
	}

	redisCfg := RedisConfig{
		Addr:     redisAddr,
		Password: redisPass,
	}

	cfg := Config{
		Database: &dbCfg,
		API:      &apiCfg,
		Storage:  &storageCfg,
		Rabbit:   &rabbitCfg,
		Redis:    &redisCfg,
	}

	return &cfg
}
