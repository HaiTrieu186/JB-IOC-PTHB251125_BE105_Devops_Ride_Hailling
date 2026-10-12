package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                string
	DatabaseURL         string
	RedisAddr           string
	RedisPassword       string
	CommissionRate      int
	TopupMaxAmount      int64
	MaxDeliveryAttempts int
	PELBatchCount       int64
}

func LoadConfig() *Config {
	port := getEnv("PORT", "8005")
	dbURL := getEnv("DATABASE_URL", "postgres://payment_user:payment_pass@postgres:5432/paymentdb?sslmode=disable")
	redisAddr := getEnv("REDIS_ADDR", "redis:6379")
	redisPassword := getEnv("REDIS_PASSWORD", "redis_secret_pass")

	commissionRate := getEnvAsInt("COMMISSION_RATE", 15)
	topupMaxAmount := getEnvAsInt64("TOPUP_MAX_AMOUNT", 100000000)
	maxDeliveryAttempts := getEnvAsInt("MAX_DELIVERY_ATTEMPTS", 5)
	pelBatchCount := getEnvAsInt64("PEL_BATCH_COUNT", 10)

	return &Config{
		Port:                port,
		DatabaseURL:         dbURL,
		RedisAddr:           redisAddr,
		RedisPassword:       redisPassword,
		CommissionRate:      commissionRate,
		TopupMaxAmount:      topupMaxAmount,
		MaxDeliveryAttempts: maxDeliveryAttempts,
		PELBatchCount:       pelBatchCount,
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsInt64(key string, defaultVal int64) int64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseInt(valStr, 10, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
