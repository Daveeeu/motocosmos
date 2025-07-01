// File: /main.go
package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"motocosmos-api/config"
	"motocosmos-api/database"
	"motocosmos-api/routes"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db, err := database.Initialize(cfg.DatabaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Run migrations
	if err := database.Migrate(db); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// Seed database with test data (optional - for development)
	if err := database.SeedData(db); err != nil {
		log.Printf("Warning: Failed to seed database: %v", err)
	}

	// Set Gin mode based on environment
	if cfg.Port == "8080" { // Development
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create router
	router := gin.Default()

	// Setup CORS middleware
	router.Use(routes.SetupCORS())

	// Request logging middleware
	router.Use(gin.Logger())

	// Recovery middleware
	router.Use(gin.Recovery())

	// Setup routes
	routes.SetupRoutes(router, db, cfg.JWTSecret)

	// Start server
	log.Printf("Starting MotoCosmos API server on port %s", cfg.Port)
	log.Printf("API Documentation available at: http://localhost:%s/api/v1/docs", cfg.Port)
	log.Printf("Health check available at: http://localhost:%s/api/v1/health", cfg.Port)

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
