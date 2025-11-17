// File: /config/config.go
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	MapboxToken string

	// Email Configuration
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

func Load() *Config {
    smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "1026"))
    return &Config{
        Port:        getEnv("PORT", "8080"),
        DatabaseURL: getEnv("DATABASE_URL", "user:password@tcp(localhost:3306)/motocosmos?charset=utf8mb4&parseTime=True&loc=Local"),
        JWTSecret:   getEnv("JWT_SECRET", "your-secret-key"),
        MapboxToken: getEnv("MAPBOX_TOKEN", "your-mapbox-token"),

        // Email settings for Mailhog in dev environment
        SMTPHost:     getEnv("SMTP_HOST", "mailhog"),
        SMTPPort:     smtpPort,
        SMTPUsername: getEnv("SMTP_USERNAME", ""), // Mailhog nem kér user/pass-t
        SMTPPassword: getEnv("SMTP_PASSWORD", ""),
        FromEmail:    getEnv("FROM_EMAIL", "dev@motocosmos.local"),
        FromName:     getEnv("FROM_NAME", "MotoCosmos Dev"),
    }
}


func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
