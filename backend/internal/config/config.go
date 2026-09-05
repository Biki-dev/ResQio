package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	JWTExpiration time.Duration
}

func Load() *Config {
	// Load .env file if present, checking current dir, backend/.env, and root
	if err := godotenv.Load(".env", "backend/.env", "../.env", "../backend/.env"); err != nil {
		log.Println("[Config] No .env file found, reading from environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://knibirdgautam@localhost:5432/auth_db?sslmode=disable"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-development-secret-key-change-in-production-12345"
	}

	jwtExpStr := os.Getenv("JWT_EXPIRATION")
	jwtExp := 24 * time.Hour
	if jwtExpStr != "" {
		if parsed, err := time.ParseDuration(jwtExpStr); err == nil {
			jwtExp = parsed
		}
	}

	return &Config{
		Port:          port,
		DatabaseURL:   dbURL,
		JWTSecret:     jwtSecret,
		JWTExpiration: jwtExp,
	}
}
