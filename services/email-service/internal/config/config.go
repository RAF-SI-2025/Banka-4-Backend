package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

type URLConfig struct {
	FrontendBaseURL string
}

type Configuration struct {
	Env      string
	Port     string
	GrpcPort string
	SMTP     SMTPConfig
	URLs     URLConfig
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
		Env:      GetOrDefault("ENV", "development"),
		Port:     GetOrDefault("PORT", "8084"),
		GrpcPort: GetOrDefault("GRPC_PORT", "50054"),
		SMTP: SMTPConfig{
			Host: GetOrThrow("SMTP_HOST"),
			Port: GetOrDefault("SMTP_PORT", "587"),
			User: GetOrDefault("SMTP_USER", ""),
			Pass: GetOrDefault("SMTP_PASS", ""),
			From: GetOrThrow("EMAIL_FROM"),
		},
		URLs: URLConfig{
			FrontendBaseURL: GetOrDefault("FRONTEND_BASE_URL", "http://localhost:5173"),
		},
	}
}
