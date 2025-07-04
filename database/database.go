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
		&models.PostBookmark{},
		&models.Follow{},
		&models.RideRecord{},
		&models.RoutePoint{},
		&models.UserLocation{},
		&models.SavedRoute{},
		&models.TripCalculation{},
		&models.Notification{},
		&models.Comment{},
		// NEW: Shared Route models
		&models.SharedRoute{},
		&models.SharedRouteLike{},
		&models.SharedRouteBookmark{},
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

	// Shared Routes indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_routes_creator_created ON shared_routes(creator_id, created_at DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_routes: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_routes_difficulty ON shared_routes(difficulty)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_routes difficulty: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_routes_title ON shared_routes(title)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_routes title: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_routes_creator_name ON shared_routes(creator_name)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_routes creator_name: %v\n", err)
	}

	// Shared Route likes composite index
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_route_likes_route_user ON shared_route_likes(route_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_route_likes: %v\n", err)
	}

	// Shared Route bookmarks composite index
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_route_bookmarks_route_user ON shared_route_bookmarks(route_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_route_bookmarks: %v\n", err)
	}

	// Shared Route bookmarks by user for bookmarked routes list
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_shared_route_bookmarks_user_created ON shared_route_bookmarks(user_id, created_at DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for shared_route_bookmarks user list: %v\n", err)
	}

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

	// NEW: Shared Route constraints
	// Prevent duplicate shared route likes
	if err := db.Exec("ALTER TABLE shared_route_likes ADD CONSTRAINT uk_shared_route_likes_route_user UNIQUE (route_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for shared_route_likes: %v\n", err)
	}

	// Prevent duplicate shared route bookmarks
	if err := db.Exec("ALTER TABLE shared_route_bookmarks ADD CONSTRAINT uk_shared_route_bookmarks_route_user UNIQUE (route_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for shared_route_bookmarks: %v\n", err)
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

	// Create sample shared routes for testing
	testSharedRoutes := []models.SharedRoute{
		{
			ID:                "route-1",
			Title:             "Mountain passes",
			Description:       "Description. Lorem ipsum dolor sit amet consectetur adipiscing elit, sed do",
			CreatorID:         "user-1",
			CreatorName:       "John Doe",
			CreatorAvatar:     "JD",
			ImageUrls:         models.StringSlice{"https://picsum.photos/300/200?random=1"},
			TotalDistance:     25.5,
			TotalElevation:    1200,
			EstimatedDuration: 7200, // 2 hours in seconds
			Difficulty:        "Hard",
			Tags:              models.StringSlice{"mountain", "scenic", "challenging"},
			LikesCount:        15,
			CommentsCount:     3,
			DownloadsCount:    8,
		},
		{
			ID:                "route-2",
			Title:             "Coastal pathway",
			Description:       "Breathtaking coastal route with ocean views and hidden beaches perfect for a day trip.",
			CreatorID:         "user-2",
			CreatorName:       "Jane Smith",
			CreatorAvatar:     "JS",
			ImageUrls:         models.StringSlice{"https://picsum.photos/300/200?random=2"},
			TotalDistance:     15.2,
			TotalElevation:    300,
			EstimatedDuration: 3600, // 1 hour in seconds
			Difficulty:        "Easy",
			Tags:              models.StringSlice{"coastal", "beach", "easy"},
			LikesCount:        23,
			CommentsCount:     7,
			DownloadsCount:    12,
		},
	}

	for _, route := range testSharedRoutes {
		if err := db.Create(&route).Error; err != nil {
			fmt.Printf("Warning: Could not create test shared route %s: %v\n", route.Title, err)
		}
	}

	fmt.Println("Database seeded with test data including shared routes")
	return nil
}
