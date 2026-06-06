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

type URLConfig struct {
	FrontendBaseURL string
	BackendBaseURL  string
}

type RedisConfig struct {
	Addr              string
	Password          string
	DB                int
	WorkCodesCacheTTL time.Duration
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type Configuration struct {
	Env                  string
	Port                 string
	DB                   DBConfig
	JWTSecret            string
	GrpcPort             string
	UserServiceAddr      string
	UserServiceBaseURL   string
	EmailServiceAddr     string
	InterbankServiceAddr string
	ExchangeRateAPIKey   string
	URLs                 URLConfig
	Redis                RedisConfig
}

func GetDurationOrDefault(env string, defaultValue time.Duration) time.Duration {
	value, ok := os.LookupEnv(env)
	if !ok {
		return defaultValue
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}

	return duration
}

func GetAsIntOrDefault(env string, defaultValue int) int {
	value, ok := os.LookupEnv(env)
	if !ok {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

func GetOrDefault(env string, defaultValue string) string {
	if value, ok := os.LookupEnv(env); ok {
		return value
	}

	return defaultValue
}

func GetOrThrow(env string) string {
	if value, ok := os.LookupEnv(env); ok {
		return value
	}

	log.Fatalf("required environment variable %q is not set", env)
	return ""
}

func Load() *Configuration {
	_ = godotenv.Load()

	return &Configuration{
		Env:                  GetOrDefault("ENV", "development"),
		Port:                 GetOrDefault("PORT", "8081"),
		GrpcPort:             GetOrDefault("GRPC_PORT", "50052"),
		JWTSecret:            GetOrThrow("JWT_SECRET"),
		UserServiceAddr:      GetOrDefault("USER_SERVICE_ADDR", "localhost:50051"),
		UserServiceBaseURL:   GetOrDefault("USER_SERVICE_BASE_URL", "http://localhost:8080"),
		EmailServiceAddr:     GetOrDefault("EMAIL_SERVICE_ADDR", "localhost:50054"),
		InterbankServiceAddr: GetOrDefault("INTERBANK_SERVICE_ADDR", "localhost:50054"),
		ExchangeRateAPIKey:   GetOrThrow("EXCHANGE_RATE_API_KEY"),
		DB: DBConfig{
			Host:     GetOrThrow("DB_HOST"),
			Port:     GetOrThrow("DB_PORT"),
			User:     GetOrThrow("DB_USER"),
			Password: GetOrThrow("DB_PASS"),
			DBName:   GetOrThrow("DB_NAME"),
			SSLMode:  GetOrDefault("DB_SSLMODE", "disable"),
		},
		URLs: URLConfig{
			FrontendBaseURL: GetOrDefault("FRONTEND_BASE_URL", "http://localhost:5173"),
			BackendBaseURL:  GetOrDefault("BACKEND_BASE_URL", "http://localhost:8081"),
		},
		Redis: RedisConfig{
			Addr:              GetOrDefault("REDIS_ADDR", "redis:6379"),
			Password:          GetOrDefault("REDIS_PASSWORD", ""),
			DB:                GetAsIntOrDefault("REDIS_DB", 0),
			WorkCodesCacheTTL: GetDurationOrDefault("WORK_CODES_CACHE_TTL", 24*time.Hour),
		},
	}
}
