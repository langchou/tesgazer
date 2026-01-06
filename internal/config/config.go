package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	ServerPort string
	Debug      bool

	// Database
	DatabaseURL string

	// Tesla API
	TeslaAuthHost    string
	TeslaAPIHost     string
	TeslaClientID    string
	TeslaRedirectURI string

	// Polling
	PollIntervalOnline  time.Duration
	PollIntervalAsleep  time.Duration
	PollIntervalCharging time.Duration

	// Token 存储路径
	TokenFile string
}

func Load() (*Config, error) {
	// 尝试加载 .env 文件（可选）
	_ = godotenv.Load()

	cfg := &Config{
		ServerPort:          getEnv("PORT", "4000"),
		Debug:               getEnvBool("DEBUG", false),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/teslamate_go?sslmode=disable"),
		TeslaAuthHost:       getEnv("TESLA_AUTH_HOST", "https://auth.tesla.com"),
		TeslaAPIHost:        getEnv("TESLA_API_HOST", "https://owner-api.teslamotors.com"),
		TeslaClientID:       getEnv("TESLA_CLIENT_ID", "ownerapi"),
		TeslaRedirectURI:    getEnv("TESLA_REDIRECT_URI", "https://auth.tesla.com/void/callback"),
		PollIntervalOnline:  getEnvDuration("POLL_INTERVAL_ONLINE", 10*time.Second),
		PollIntervalAsleep:  getEnvDuration("POLL_INTERVAL_ASLEEP", 60*time.Second),
		PollIntervalCharging: getEnvDuration("POLL_INTERVAL_CHARGING", 30*time.Second),
		TokenFile:           getEnv("TOKEN_FILE", "tokens.json"),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		d, err := time.ParseDuration(value)
		if err == nil {
			return d
		}
	}
	return defaultValue
}
