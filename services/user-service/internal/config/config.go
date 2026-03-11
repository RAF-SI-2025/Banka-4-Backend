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

func (c *DBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", c.Host, c.Port, c.User, c.Password, c.DBName)
}

type Configuration struct {
	Env       string
	Port      string
	DB        DBConfig
	GrpcPort  string // unused for now until we add multiple microservices
	JWTSecret string // Dodato za JWT
	JWTExpiry int    // U minutima
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

	expiryStr := GetOrDefault("JWT_EXPIRY_HOURS", "24")
	expiry, _ := strconv.Atoi(expiryStr)
	return &Configuration{
		Env:  GetOrDefault("ENV", "development"),
		Port: GetOrDefault("PORT", "8080"),
		DB: DBConfig{
			Host:     GetOrThrow("DB_HOST"),
			Port:     GetOrThrow("DB_PORT"),
			User:     GetOrThrow("DB_USER"),
			Password: GetOrThrow("DB_PASS"),
			DBName:   GetOrThrow("DB_NAME"),
		},
		JWTSecret: GetOrThrow("JWT_SECRET"),
		JWTExpiry: expiry,
	}
}
