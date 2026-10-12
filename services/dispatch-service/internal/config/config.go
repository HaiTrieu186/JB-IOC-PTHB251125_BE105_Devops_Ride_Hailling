package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	RedisAddr              string
	RedisPassword          string
	CommissionRate         int
	OfferRadiusKM          float64
	MatchingTimeoutSeconds int
	InternalHTTPTimeoutMS  int
	PricingServiceURL      string
	PaymentServiceURL      string
	LocationServiceURL     string
	UserServiceURL         string
}

func LoadConfig() *Config {
	return &Config{
		Port:                   getEnv("PORT", "8003"),
		DatabaseURL:            getEnv("DATABASE_URL", "postgres://dispatch_user:dispatch_pass@postgres:5432/dispatchdb?sslmode=disable"),
		RedisAddr:              getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:          getEnv("REDIS_PASSWORD", "redis_secret_pass"),
		CommissionRate:         getEnvAsInt("COMMISSION_RATE", 15),
		OfferRadiusKM:          getEnvAsFloat("OFFER_RADIUS_KM", 5.0),
		MatchingTimeoutSeconds: getEnvAsInt("MATCHING_TIMEOUT_SECONDS", 30),
		InternalHTTPTimeoutMS:  getEnvAsInt("INTERNAL_HTTP_TIMEOUT_MS", 2000),
		PricingServiceURL:      getEnv("PRICING_SERVICE_URL", "http://pricing-service:8004"),
		PaymentServiceURL:      getEnv("PAYMENT_SERVICE_URL", "http://payment-service:8005"),
		LocationServiceURL:     getEnv("LOCATION_SERVICE_URL", "http://location-service:8002"),
		UserServiceURL:         getEnv("USER_SERVICE_URL", "http://user-service:8001"),
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

func getEnvAsFloat(key string, defaultVal float64) float64 {
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
