package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	JWTSecret          string
	RedisAddr          string
	RedisPassword      string
	UserServiceURL     string
	LocationServiceURL string
	DispatchServiceURL string
	PricingServiceURL  string
	PaymentServiceURL  string
	AIServiceURL       string
	ProxyTimeoutMS     int
}

func LoadConfig() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", "secret-key-ride-hailing-devops"),
		RedisAddr:          getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", "redis_secret_pass"),
		UserServiceURL:     getEnv("USER_SERVICE_URL", "http://user-service:8001"),
		LocationServiceURL: getEnv("LOCATION_SERVICE_URL", "http://location-service:8002"),
		DispatchServiceURL: getEnv("DISPATCH_SERVICE_URL", "http://dispatch-service:8003"),
		PricingServiceURL:  getEnv("PRICING_SERVICE_URL", "http://pricing-service:8004"),
		PaymentServiceURL:  getEnv("PAYMENT_SERVICE_URL", "http://payment-service:8005"),
		AIServiceURL:       getEnv("AI_SERVICE_URL", "http://ai-service:8006"),
		ProxyTimeoutMS:     getEnvInt("PROXY_TIMEOUT_MS", 15000),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}
