package config

import (
	"os"
)

type Config struct {
	Port      string
	RedisAddr string
	RedisPass string
}

func Load() *Config {
	port := os.Getenv("CLAIM_SERVICE_PORT")
	if port == "" {
		port = "8082"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	return &Config{
		Port:      port,
		RedisAddr: redisAddr,
		RedisPass: os.Getenv("REDIS_PASSWORD"),
	}
}
