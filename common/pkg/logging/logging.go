package logging

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var log *zap.Logger

func Error(msg string, fields ...zap.Field) {
	log.Error(msg, fields...)
}

// Init initializes the global logger based on the environment.
// In production, it uses a JSON logger optimized for log aggregation tools.
// In all other environments, it uses a human-readable logger suitable for development.
func Init(env string) error {
	var err error
	if env == "production" {
		log, err = zap.NewProduction()
	} else {
		log, err = zap.NewDevelopment()
	}

	return err
}

// Logger returns a Gin middleware that logs incoming HTTP requests.
// It logs the request method, path, status code, duration, and client IP.
//
// This logger should be used for each individual microservice.
//
// Example Usage:
//
//	r := gin.New()
//	r.Use(logging.Logger())
func Logger() gin.HandlerFunc {
	return func(context *gin.Context) {
		start := time.Now()
		context.Next()

		log.Info("request",
			zap.String("method", context.Request.Method),
			zap.String("path", context.Request.URL.Path),
			zap.Int("status", context.Writer.Status()),
			zap.String("duration", fmt.Sprintf("%dms", time.Since(start).Milliseconds())),
			zap.String("ip", context.ClientIP()),
		)
	}
}
