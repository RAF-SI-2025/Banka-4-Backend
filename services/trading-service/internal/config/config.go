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
	SSLMode  string
}

type URLConfig struct {
	FrontendBaseURL string
	BackendBaseURL  string
}

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}

type Configuration struct {
	Env                   string
	Port                  string
	DB                    DBConfig
	JWTSecret             string
	GrpcPort              string // reserved for future banking-service gRPC endpoints
	UserServiceAddr       string
	UserServiceBaseURL    string
	BankingServiceAddr    string
	URLs                  URLConfig
	ExchangeRateAPIKey    string
	FinnhubAPIKey         string
	TaxAccountNumber      string
	DividendAccountNumber string
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
		Port:               GetOrDefault("PORT", "8082"),
		GrpcPort:           GetOrDefault("GRPC_PORT", "50053"),
		JWTSecret:          GetOrThrow("JWT_SECRET"),
		UserServiceAddr:    GetOrDefault("USER_SERVICE_ADDR", "localhost:50051"),
		UserServiceBaseURL: GetOrDefault("USER_SERVICE_BASE_URL", "http://localhost:8080"),
		BankingServiceAddr: GetOrDefault("BANKING_SERVICE_ADDR", "localhost:50052"),
		FinnhubAPIKey:      GetOrThrow("FINNHUB_API_KEY"),
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
			BackendBaseURL:  GetOrDefault("BACKEND_BASE_URL", "http://localhost:8082"),
		},
		ExchangeRateAPIKey:    GetOrThrow("EXCHANGE_RATE_API_KEY"),
		TaxAccountNumber:      GetOrDefault("TAX_ACCOUNT_NUMBER", "444000000000000008"),
		DividendAccountNumber: GetOrDefault("DIVIDEND_ACCOUNT_NUMBER", "444000000000000009"),
	}
}
