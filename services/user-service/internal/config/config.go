package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
}

type URLConfig struct {
	FrontendBaseURL string
	BackendBaseURL  string
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", c.Host, c.Port, c.User, c.Password, c.DBName)
}

type Configuration struct {
	Env                string
	Port               string
	DB                 DBConfig
	GrpcPort           string
	TradingServiceAddr string
	EmailServiceAddr   string
	JWTSecret          string
	JWTExpiry          int
	URLs               URLConfig
	RefreshExpiry      int
	FailedLoginWindow  int
	MaxFailedLogins    int
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
		Env:                GetOrDefault("ENV", "development"),
		Port:               GetOrDefault("PORT", "8080"),
		GrpcPort:           GetOrDefault("GRPC_PORT", "50051"),
		TradingServiceAddr: GetOrDefault("TRADING_SERVICE_ADDR", "localhost:50053"),
		EmailServiceAddr:   GetOrDefault("EMAIL_SERVICE_ADDR", "localhost:50054"),
		DB: DBConfig{
			Host:     GetOrThrow("DB_HOST"),
			Port:     GetOrThrow("DB_PORT"),
			User:     GetOrThrow("DB_USER"),
			Password: GetOrThrow("DB_PASS"),
			DBName:   GetOrThrow("DB_NAME"),
		},
		JWTSecret:         GetOrThrow("JWT_SECRET"),
		JWTExpiry:         GetAsIntOrDefault("JWT_EXPIRY", 15),
		RefreshExpiry:     GetAsIntOrDefault("REFRESH_EXPIRY_MINUTES", 10080),
		FailedLoginWindow: GetAsIntOrDefault("FAILED_LOGIN_WINDOW_MINUTES", 5),
		MaxFailedLogins:   GetAsIntOrDefault("MAX_FAILED_LOGINS", 4),
		URLs: URLConfig{
			FrontendBaseURL: GetOrDefault("FRONTEND_BASE_URL", "http://localhost:5173"),
			BackendBaseURL:  GetOrDefault("BACKEND_BASE_URL", "http://localhost:8080"),
		},
	}
}
