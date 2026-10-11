package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                string
	RedisAddr           string
	RedisPassword       string
	DefaultRadiusKM     float64
	MaxRadiusKM         float64
	GPSFreshnessSeconds int64
	GeoScanCount        int
}

func LoadConfig() *Config {
	return &Config{
		Port:                getEnv("PORT", "8002"),
		RedisAddr:           getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:       getEnv("REDIS_PASSWORD", "redis_secret_pass"),
		DefaultRadiusKM:     getEnvFloat("DEFAULT_RADIUS_KM", 5.0),
		MaxRadiusKM:         getEnvFloat("MAX_RADIUS_KM", 10.0),
		GPSFreshnessSeconds: getEnvInt64("GPS_FRESHNESS_SECONDS", 15),
		GeoScanCount:        getEnvInt("GEO_SCAN_COUNT", 500),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvInt64(key string, defaultVal int64) int64 {
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

func getEnvInt(key string, defaultVal int) int {
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
