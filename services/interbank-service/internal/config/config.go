package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type URLConfig struct {
	FrontendBaseURL string
	BackendBaseURL  string
}

type Configuration struct {
	Env               string
	Port              string
	DB                DBConfig
	URLs              URLConfig
	OurRoutingNumber  int
	PeersConfigPath   string
	OutboundHTTPTO    time.Duration
	OutboxPollEvery   time.Duration
	OutboxMaxAttempts int
}

func getOrDefault(env string, def string) string {
	if v, ok := os.LookupEnv(env); ok {
		return v
	}
	return def
}

func getOrThrow(env string) string {
	if v, ok := os.LookupEnv(env); ok {
		return v
	}
	log.Fatalf("required environment variable %q is not set", env)
	return ""
}

func getIntOrDefault(env string, def int) int {
	v, ok := os.LookupEnv(env)
	if !ok {
		return def
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getDurationOrDefault(env string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(env)
	if !ok {
		return def
	}

	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func Load() *Configuration {
	_ = godotenv.Load()

	return &Configuration{
		Env:               getOrDefault("ENV", "development"),
		Port:              getOrDefault("PORT", "8083"),
		OurRoutingNumber:  getIntOrDefault("INTERBANK_ROUTING_NUMBER", 444),
		PeersConfigPath:   getOrDefault("INTERBANK_PEERS_CONFIG_PATH", "./peers.yaml"),
		OutboundHTTPTO:    getDurationOrDefault("INTERBANK_OUTBOUND_HTTP_TIMEOUT", 10*time.Second),
		OutboxPollEvery:   getDurationOrDefault("INTERBANK_OUTBOX_POLL_INTERVAL", 2*time.Second),
		OutboxMaxAttempts: getIntOrDefault("INTERBANK_OUTBOX_MAX_ATTEMPTS", 20),
		DB: DBConfig{
			Host:     getOrThrow("DB_HOST"),
			Port:     getOrThrow("DB_PORT"),
			User:     getOrThrow("DB_USER"),
			Password: getOrThrow("DB_PASS"),
			DBName:   getOrThrow("DB_NAME"),
			SSLMode:  getOrDefault("DB_SSLMODE", "disable"),
		},
		URLs: URLConfig{
			FrontendBaseURL: getOrDefault("FRONTEND_BASE_URL", "http://localhost:5173"),
			BackendBaseURL:  getOrDefault("BACKEND_BASE_URL", "http://localhost:8083"),
		},
	}
}
