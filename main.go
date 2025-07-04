// File: /main.go
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
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
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Handle command line arguments
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			fmt.Println("Running database migrations...")
			if err := database.Migrate(db); err != nil {
				log.Fatalf("Migration failed: %v", err)
			}
			fmt.Println("Migrations completed successfully!")
			return
		case "seed":
			fmt.Println("Seeding database with test data...")
			if err := database.SeedData(db); err != nil {
				log.Fatalf("Seeding failed: %v", err)
			}
			fmt.Println("Database seeded successfully!")
			return
		}
	}

	// Auto-migrate database (for development)
	if gin.Mode() == gin.DebugMode {
		fmt.Println("Running auto-migration in debug mode...")
		if err := database.Migrate(db); err != nil {
			log.Fatalf("Auto-migration failed: %v", err)
		}
	}

	// Initialize Gin router
	router := gin.Default()

	// Setup CORS
	router.Use(routes.SetupCORS())

	// Setup routes
	routes.SetupRoutes(router, db, cfg.JWTSecret)

	// Start server
	fmt.Printf("🚀 MotoCosmos API Server starting on port %s\n", cfg.Port)
	fmt.Printf("📚 API Documentation: http://localhost:%s/api/v1/docs\n", cfg.Port)
	fmt.Printf("💚 Health Check: http://localhost:%s/api/v1/health\n", cfg.Port)

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
