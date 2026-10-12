package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                string
	RedisAddr           string
	RedisPassword       string
	JWTSecret           string
	LocationServiceURL  string
	LocationTimeoutMS   int
	PingIntervalSeconds int
	PongWaitSeconds     int
	ClientBufferSize    int
	MaxConnections      int
}

func LoadConfig() *Config {
	return &Config{
		Port:                getEnv("PORT", "8081"),
		RedisAddr:           getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:       getEnv("REDIS_PASSWORD", "redis_secret_pass"),
		JWTSecret:           getEnv("JWT_SECRET", "secret-key-ride-hailing-devops"),
		LocationServiceURL:  getEnv("LOCATION_SERVICE_URL", "http://location-service:8002"),
		LocationTimeoutMS:   getEnvInt("LOCATION_TIMEOUT_MS", 300),
		PingIntervalSeconds: getEnvInt("PING_INTERVAL_SECONDS", 30),
		PongWaitSeconds:     getEnvInt("PONG_WAIT_SECONDS", 60),
		ClientBufferSize:    getEnvInt("CLIENT_BUFFER_SIZE", 64),
		MaxConnections:      getEnvInt("MAX_CONNECTIONS", 300),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
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
