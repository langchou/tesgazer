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

	// Polling - 基础间隔
	PollIntervalOnline   time.Duration
	PollIntervalAsleep   time.Duration
	PollIntervalCharging time.Duration
	PollIntervalDriving  time.Duration

	// Polling - 指数退避参数
	PollBackoffInitial time.Duration // 初始退避间隔
	PollBackoffMax     time.Duration // 最大退避间隔
	PollBackoffFactor  float64       // 退避因子 (通常为 2)

	// Sleep/Suspend 配置
	SuspendAfterIdleMin int           // 空闲多少分钟后自动暂停 (默认 15 分钟)
	SuspendPollInterval time.Duration // 暂停状态下的轮询间隔 (默认 21 分钟)
	RequireNotUnlocked  bool          // 是否要求车辆必须锁定才能休眠

	// Tesla Streaming API 配置 (双链路架构)
	UseStreamingAPI         bool          // 是否启用 Streaming API
	StreamingHost           string        // Streaming WebSocket 地址
	StreamingReconnectDelay time.Duration // 重连延迟

	// 高德地图 API 配置 (用于逆地理编码)
	AmapAPIKey string // 高德 Web 服务 API Key

	// Token 存储路径
	TokenFile string
}

func Load() (*Config, error) {
	// 尝试加载 .env 文件（可选）
	_ = godotenv.Load()

	cfg := &Config{
		ServerPort:              getEnv("PORT", "4000"),
		Debug:                   getEnvBool("DEBUG", false),
		DatabaseURL:             getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/tesgazer?sslmode=disable"),
		TeslaAuthHost:           getEnv("TESLA_AUTH_HOST", "https://auth.tesla.com"),
		TeslaAPIHost:            getEnv("TESLA_API_HOST", "https://owner-api.teslamotors.com"),
		TeslaClientID:           getEnv("TESLA_CLIENT_ID", "ownerapi"),
		TeslaRedirectURI:        getEnv("TESLA_REDIRECT_URI", "https://auth.tesla.com/void/callback"),
		PollIntervalOnline:      getEnvDuration("POLL_INTERVAL_ONLINE", 15*time.Second),
		PollIntervalAsleep:      getEnvDuration("POLL_INTERVAL_ASLEEP", 30*time.Second),
		PollIntervalCharging:    getEnvDuration("POLL_INTERVAL_CHARGING", 5*time.Second),
		PollIntervalDriving:     getEnvDuration("POLL_INTERVAL_DRIVING", 3*time.Second),
		PollBackoffInitial:      getEnvDuration("POLL_BACKOFF_INITIAL", 1*time.Second),
		PollBackoffMax:          getEnvDuration("POLL_BACKOFF_MAX", 30*time.Second),
		PollBackoffFactor:       getEnvFloat("POLL_BACKOFF_FACTOR", 2.0),
		SuspendAfterIdleMin:     getEnvInt("SUSPEND_AFTER_IDLE_MIN", 15),
		SuspendPollInterval:     getEnvDuration("SUSPEND_POLL_INTERVAL", 21*time.Minute),
		RequireNotUnlocked:      getEnvBool("REQUIRE_NOT_UNLOCKED", false),
		UseStreamingAPI:         getEnvBool("USE_STREAMING_API", true), // 默认启用
		StreamingHost:           getEnv("STREAMING_HOST", "wss://streaming.vn.cloud.tesla.cn/streaming/"), // 中国区域名
		StreamingReconnectDelay: getEnvDuration("STREAMING_RECONNECT_DELAY", 5*time.Second),
		AmapAPIKey:              getEnv("AMAP_API_KEY", ""), // 高德地图 API Key
		TokenFile:               getEnv("TOKEN_FILE", "tokens.json"),
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

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		f, err := strconv.ParseFloat(value, 64)
		if err == nil {
			return f
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return defaultValue
}
