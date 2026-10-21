package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port               string
	PostgresHost       string
	PostgresPort       int
	PostgresUser       string
	PostgresPass       string
	PostgresDB         string
	PostgresSSL        string
	RedisAddr          string
	RedisPass          string
	JWTSecret          string
	CustomerServiceURL string
	InternalAPIKey     string
	EnableDemoOTP      bool
}

func Load() *Config {
	port := os.Getenv("AUTH_SERVICE_PORT")
	if port == "" {
		port = "8084"
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
		pgPass = "postgrespassword"
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "salt-hackathon-jwt-secret-key-32bytes"
	}

	customerURL := os.Getenv("CUSTOMER_SERVICE_URL")
	if customerURL == "" {
		customerURL = "http://localhost:8085"
	}

	apiKey := os.Getenv("INTERNAL_API_KEY")
	if apiKey == "" {
		apiKey = "salt-hackathon-internal-secret"
	}

	enableDemoOTP := true
	if val := os.Getenv("ENABLE_DEMO_OTP"); val == "false" {
		enableDemoOTP = false
	}

	return &Config{
		Port:               port,
		PostgresHost:       pgHost,
		PostgresPort:       pgPort,
		PostgresUser:       pgUser,
		PostgresPass:       pgPass,
		PostgresDB:         pgDB,
		PostgresSSL:        pgSSL,
		RedisAddr:          redisAddr,
		RedisPass:          os.Getenv("REDIS_PASSWORD"),
		JWTSecret:          jwtSecret,
		CustomerServiceURL: customerURL,
		InternalAPIKey:     apiKey,
		EnableDemoOTP:      enableDemoOTP,
	}
}
