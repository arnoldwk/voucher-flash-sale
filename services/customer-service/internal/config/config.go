package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port           string
	PostgresHost   string
	PostgresPort   int
	PostgresUser   string
	PostgresPass   string
	PostgresDB     string
	PostgresSSL    string
	InternalAPIKey string
}

func Load() *Config {
	port := os.Getenv("CUSTOMER_SERVICE_PORT")
	if port == "" {
		port = "8085"
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

	apiKey := os.Getenv("INTERNAL_API_KEY")
	if apiKey == "" {
		apiKey = "salt-hackathon-internal-secret"
	}

	return &Config{
		Port:           port,
		PostgresHost:   pgHost,
		PostgresPort:   pgPort,
		PostgresUser:   pgUser,
		PostgresPass:   pgPass,
		PostgresDB:     pgDB,
		PostgresSSL:    pgSSL,
		InternalAPIKey: apiKey,
	}
}
