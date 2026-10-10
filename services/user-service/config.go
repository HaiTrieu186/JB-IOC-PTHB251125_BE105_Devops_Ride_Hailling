package main

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	RedisAddr          string
	RedisPassword      string
	JWTSecret          string
	AccessTokenExpiry  int
	RefreshTokenExpiry int
	AdminEmail         string
	AdminPassword      string
}

func loadConfig() *Config {
	return &Config{
		Port:               getEnv("PORT", "8001"),
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://user_user:user_pass@postgres:5432/userdb?sslmode=disable"),
		RedisAddr:          getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", "redis_secret_pass"),
		JWTSecret:          getEnv("JWT_SECRET", "secret-key-ride-hailing-devops"),
		AccessTokenExpiry:  getEnvInt("ACCESS_TOKEN_EXPIRY", 3600),
		RefreshTokenExpiry: getEnvInt("REFRESH_TOKEN_EXPIRY", 604800),
		AdminEmail:         getEnv("ADMIN_EMAIL", "admin@ridehailing.local"),
		AdminPassword:      getEnv("ADMIN_PASSWORD", "Admin@123456"),
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
