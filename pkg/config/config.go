package config

import (
	"os"
)

// AppConfig holds centralized environment and CLI configuration.
type AppConfig struct {
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	StreamName    string
	ConsumerGroup string
	DefaultCodec  string
	DefaultRes    string
	OutputDir     string
}

// LoadConfig loads configuration with sensible defaults.
func LoadConfig() *AppConfig {
	return &AppConfig{
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       0,
		StreamName:    getEnv("PROXY_STREAM", "proxy:jobs"),
		ConsumerGroup: getEnv("PROXY_GROUP", "proxy_workers"),
		DefaultCodec:  getEnv("PROXY_CODEC", "prores"), // "prores" or "h264"
		DefaultRes:    getEnv("PROXY_RES", "1080p"),    // "1080p" or "720p"
		OutputDir:     getEnv("PROXY_OUTPUT_DIR", "./proxies"),
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
