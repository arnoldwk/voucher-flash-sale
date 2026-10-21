package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                  string
	PostgresHost          string
	PostgresPort          int
	PostgresUser          string
	PostgresPass          string
	PostgresDB            string
	PostgresSSL           string
	RedisAddr             string
	RedisPass             string
	JWTSecret             string
	ReservationTTLSeconds int
	ExpiryWorkerInterval  time.Duration
}

func Load() *Config {
	port := os.Getenv("TRANSACTION_SERVICE_PORT")
	if port == "" {
		port = "8086"
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

	reservationTTL := 5 * time.Minute
	if v := os.Getenv("RESERVATION_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			reservationTTL = time.Duration(n) * time.Second
		}
	}

	expiryInterval := 10 * time.Second
	if v := os.Getenv("EXPIRY_WORKER_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			expiryInterval = time.Duration(n) * time.Second
		}
	}

	return &Config{
		Port:                  port,
		PostgresHost:          pgHost,
		PostgresPort:          pgPort,
		PostgresUser:          pgUser,
		PostgresPass:          pgPass,
		PostgresDB:            pgDB,
		PostgresSSL:           pgSSL,
		RedisAddr:             redisAddr,
		RedisPass:             os.Getenv("REDIS_PASSWORD"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		ReservationTTLSeconds: int(reservationTTL.Seconds()),
		ExpiryWorkerInterval:  expiryInterval,
	}
}
