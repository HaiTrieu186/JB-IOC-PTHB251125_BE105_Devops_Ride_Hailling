package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	RedisAddr             string
	RedisPassword         string
	DefaultBaseFare       int64
	DefaultPricePerKm     int64
	SurgeThreshold        float64
	OSRMBaseURL           string
	OSRMTimeoutMs         int
	InternalHTTPTimeoutMs int
	LocationServiceURL    string
	UserServiceURL        string
	MaxDeliveryAttempts   int64
	PELBatchCount         int64
	GOMEMLIMIT            string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:                  getEnv("PORT", "8004"),
		DatabaseURL:           getEnv("DATABASE_URL", "postgres://pricing_user:pricing_pass@postgres:5432/pricingdb?sslmode=disable"),
		RedisAddr:             getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:         getEnv("REDIS_PASSWORD", "redis_secret_pass"),
		DefaultBaseFare:       getEnvInt64("DEFAULT_BASE_FARE", 10000),
		DefaultPricePerKm:     getEnvInt64("DEFAULT_PRICE_PER_KM", 12000),
		OSRMBaseURL:           getEnv("OSRM_BASE_URL", "https://router.project-osrm.org"),
		OSRMTimeoutMs:         getEnvInt("OSRM_TIMEOUT_MS", 400),
		InternalHTTPTimeoutMs: getEnvInt("INTERNAL_HTTP_TIMEOUT_MS", 500),
		LocationServiceURL:    getEnv("LOCATION_SERVICE_URL", "http://location-service:8002"),
		UserServiceURL:        getEnv("USER_SERVICE_URL", "http://user-service:8001"),
		MaxDeliveryAttempts:   getEnvInt64("MAX_DELIVERY_ATTEMPTS", 5),
		PELBatchCount:         getEnvInt64("PEL_BATCH_COUNT", 10),
		GOMEMLIMIT:            getEnv("GOMEMLIMIT", "26MiB"),
	}

	surgeStr := getEnv("SURGE_THRESHOLD", "1.0")
	val, err := strconv.ParseFloat(surgeStr, 64)
	if err != nil {
		return nil, fmt.Errorf("SURGE_THRESHOLD '%s' is not a valid float: %w", surgeStr, err)
	}

	if val < 0.01 || val > 99.99 {
		return nil, fmt.Errorf("SURGE_THRESHOLD %v is out of range [0.01, 99.99]", val)
	}

	// Kiểm tra tối đa 2 chữ số thập phân
	parts := strings.Split(surgeStr, ".")
	if len(parts) == 2 && len(parts[1]) > 2 {
		return nil, fmt.Errorf("SURGE_THRESHOLD '%s' has more than 2 decimal places", surgeStr)
	}
	if math.Abs(val*100-math.Round(val*100)) > 1e-5 {
		return nil, fmt.Errorf("SURGE_THRESHOLD %v has more than 2 decimal places", val)
	}

	cfg.SurgeThreshold = math.Round(val*100) / 100

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvInt64(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultVal
}
