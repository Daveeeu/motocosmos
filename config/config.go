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
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "2525"))
	// Looking to send emails in production? Check out our Email API/SMTP product!
	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "user:password@tcp(localhost:3306)/motocosmos?charset=utf8mb4&parseTime=True&loc=Local"),
		JWTSecret:   getEnv("JWT_SECRET", "your-secret-key"),
		MapboxToken: getEnv("MAPBOX_TOKEN", "your-mapbox-token"),

		// Email settings
		SMTPHost:     getEnv("SMTP_HOST", "sandbox.smtp.mailtrap.io"),
		SMTPPort:     smtpPort,
		SMTPUsername: getEnv("SMTP_USERNAME", "42e3731f7fdc7f"),
		SMTPPassword: getEnv("SMTP_PASSWORD", "7b2249398b02a0"),
		FromEmail:    getEnv("FROM_EMAIL", "noreply@motocosmos.com"),
		FromName:     getEnv("FROM_NAME", "MotoCosmos"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
