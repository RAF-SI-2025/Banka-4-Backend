package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func New(dsn string) (*gorm.DB, error) {
	print("Hello")
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}
