// File: /database/database.go
package database

import (
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"motocosmos-api/models"
)

func Initialize(databaseURL string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(databaseURL), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Info),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	// Auto migrate all models
	err := db.AutoMigrate(
		&models.User{},
		&models.Motorcycle{},
		&models.Route{},
		&models.RouteWaypoint{},
		&models.CommunityEvent{},
		&models.EventParticipant{},
		&models.Post{},
		&models.PostLike{},
		&models.PostBookmark{}, // NEW: Bookmark model
		&models.Follow{},
		&models.RideRecord{},
		&models.RoutePoint{},
		&models.UserLocation{},
		&models.SavedRoute{},
		&models.TripCalculation{},
		&models.Notification{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	// Add custom indexes for better performance
	if err := addCustomIndexes(db); err != nil {
		return fmt.Errorf("failed to add custom indexes: %w", err)
	}

	// Add triggers or constraints if needed
	if err := addDatabaseConstraints(db); err != nil {
		return fmt.Errorf("failed to add database constraints: %w", err)
	}

	return nil
}

func addCustomIndexes(db *gorm.DB) error {
	// Add composite indexes for better query performance

	/*	// Posts feed queries
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_posts_user_created ON posts(user_id, created_at DESC)").Error; err != nil {
			return err
		}

		// Post likes composite index
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_post_likes_post_user ON post_likes(post_id, user_id)").Error; err != nil {
			return err
		}

		// Post bookmarks composite index
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_post_bookmarks_post_user ON post_bookmarks(post_id, user_id)").Error; err != nil {
			return err
		}

		// Post bookmarks by user for bookmarked posts list
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_post_bookmarks_user_created ON post_bookmarks(user_id, created_at DESC)").Error; err != nil {
			return err
		}

		// Follow relationships
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_follows_follower_following ON follows(follower_id, following_id)").Error; err != nil {
			return err
		}

		// User search indexes
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_name ON users(name)").Error; err != nil {
			return err
		}

		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_handle ON users(handle)").Error; err != nil {
			return err
		}

		// User popularity for search ranking
		if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_followers_count ON users(followers_count DESC)").Error; err != nil {
			return err
		}*/

	return nil
}

func addDatabaseConstraints(db *gorm.DB) error {
	// Add unique constraints to prevent duplicate likes/bookmarks/follows

	// Prevent duplicate likes
	if err := db.Exec("ALTER TABLE post_likes ADD CONSTRAINT uk_post_likes_post_user UNIQUE (post_id, user_id)").Error; err != nil {
		// Ignore error if constraint already exists
		fmt.Printf("Warning: Could not add unique constraint for post_likes: %v\n", err)
	}

	// Prevent duplicate bookmarks
	if err := db.Exec("ALTER TABLE post_bookmarks ADD CONSTRAINT uk_post_bookmarks_post_user UNIQUE (post_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for post_bookmarks: %v\n", err)
	}

	// Prevent duplicate follows
	if err := db.Exec("ALTER TABLE follows ADD CONSTRAINT uk_follows_follower_following UNIQUE (follower_id, following_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for follows: %v\n", err)
	}

	// Prevent self-following
	if err := db.Exec("ALTER TABLE follows ADD CONSTRAINT ck_follows_no_self_follow CHECK (follower_id != following_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add check constraint for follows: %v\n", err)
	}

	return nil
}

// SeedData can be used to populate the database with initial data for development/testing
func SeedData(db *gorm.DB) error {
	// Check if we already have users
	var userCount int64
	db.Model(&models.User{}).Count(&userCount)

	if userCount > 0 {
		fmt.Println("Database already has data, skipping seed")
		return nil
	}

	// Create sample users for testing
	testUsers := []models.User{
		{
			ID:            "user-1",
			Name:          "John Doe",
			Handle:        "john_doe",
			Email:         "john@example.com",
			Password:      "$2a$10$dummy", // This should be properly hashed in real scenarios
			EmailVerified: true,
		},
		{
			ID:            "user-2",
			Name:          "Jane Smith",
			Handle:        "jane_smith",
			Email:         "jane@example.com",
			Password:      "$2a$10$dummy",
			EmailVerified: true,
		},
	}

	for _, user := range testUsers {
		if err := db.Create(&user).Error; err != nil {
			fmt.Printf("Warning: Could not create test user %s: %v\n", user.Handle, err)
		}
	}

	fmt.Println("Database seeded with test data")
	return nil
}
