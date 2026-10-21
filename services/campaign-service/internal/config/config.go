package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port         string
	PostgresHost string
	PostgresPort int
	PostgresUser string
	PostgresPass string
	PostgresDB   string
	PostgresSSL  string
	RedisAddr    string
	RedisPass    string
}

func Load() *Config {
	port := os.Getenv("CAMPAIGN_SERVICE_PORT")
	if port == "" {
		port = "8081"
	}

	pgPort, err := strconv.Atoi(os.Getenv("POSTGRES_PORT"))
	if err != nil {
		pgPort = 5432
	}

	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "localhost"
	}

	pgUser := os.Getenv("POSTGRES_USER")
	if pgUser == "" {
		pgUser = "postgres"
	}

	pgPass := os.Getenv("POSTGRES_PASSWORD")
	if pgPass == "" {
		pgPass = "postgres"
	}

	pgDB := os.Getenv("POSTGRES_DB")
	if pgDB == "" {
		pgDB = "flashsale"
	}

	pgSSL := os.Getenv("POSTGRES_SSLMODE")
	if pgSSL == "" {
		pgSSL = "disable"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	return &Config{
		Port:         port,
		PostgresHost: pgHost,
		PostgresPort: pgPort,
		PostgresUser: pgUser,
		PostgresPass: pgPass,
		PostgresDB:   pgDB,
		PostgresSSL:  pgSSL,
		RedisAddr:    redisAddr,
		RedisPass:    os.Getenv("REDIS_PASSWORD"),
	}
}
