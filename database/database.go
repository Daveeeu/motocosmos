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
		&models.SavedRoute{},
		&models.CommunityEvent{},
		&models.EventParticipant{},
		&models.Post{},
		&models.PostLike{},
		&models.PostBookmark{},
		&models.Follow{},
		&models.RideRecord{},
		&models.RoutePoint{},
		&models.UserLocation{},
		&models.LocationVisibilitySettings{},      // ← ÚJ
		&models.LocationVisibilityAllowed{},       // ← ÚJ
		&models.TripCalculation{},
		&models.Notification{},
		&models.Comment{},
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

	// User locations indexes
db.Exec("CREATE INDEX IF NOT EXISTS idx_user_locations_user_id ON user_locations(user_id)")
db.Exec("CREATE INDEX IF NOT EXISTS idx_user_locations_online ON user_locations(is_online)")
db.Exec("CREATE INDEX IF NOT EXISTS idx_user_locations_updated ON user_locations(updated_at DESC)")
// Friend request indexes
if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_friend_requests_sender ON friend_requests(sender_id)").Error; err != nil {
	fmt.Printf("Warning: Could not create index for friend_requests sender: %v\n", err)
}

if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_friend_requests_receiver ON friend_requests(receiver_id)").Error; err != nil {
	fmt.Printf("Warning: Could not create index for friend_requests receiver: %v\n", err)
}

if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_friend_requests_status ON friend_requests(status)").Error; err != nil {
	fmt.Printf("Warning: Could not create index for friend_requests status: %v\n", err)
}

// Friendship indexes
if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_friendships_user1 ON friendships(user1_id)").Error; err != nil {
	fmt.Printf("Warning: Could not create index for friendships user1: %v\n", err)
}

if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_friendships_user2 ON friendships(user2_id)").Error; err != nil {
	fmt.Printf("Warning: Could not create index for friendships user2: %v\n", err)
}
// Location visibility indexes
db.Exec("CREATE INDEX IF NOT EXISTS idx_location_visibility_settings_user ON location_visibility_settings(user_id)")
db.Exec("CREATE INDEX IF NOT EXISTS idx_location_visibility_allowed_owner ON location_visibility_allowed(owner_user_id)")
db.Exec("CREATE INDEX IF NOT EXISTS idx_location_visibility_composite ON location_visibility_allowed(owner_user_id, allowed_user_id)")
	// NEW: Personal Routes indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_routes_user_created ON routes(user_id, created_at DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for routes: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_routes_difficulty ON routes(difficulty)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for routes difficulty: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_routes_public ON routes(is_public)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for routes public: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_routes_name ON routes(name)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for routes name: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_routes_times_used ON routes(times_used DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for routes times_used: %v\n", err)
	}

	// Route waypoints indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_route_waypoints_route_order ON route_waypoints(route_id, `order`)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for route_waypoints: %v\n", err)
	}

	// Saved routes (bookmarks) indexes
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_saved_routes_user_route ON saved_routes(user_id, route_id)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for saved_routes: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_saved_routes_user_created ON saved_routes(user_id, created_at DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for saved_routes user list: %v\n", err)
	}

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

	// Posts and other existing indexes...
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_posts_user_created ON posts(user_id, created_at DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for posts: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_post_likes_post_user ON post_likes(post_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for post_likes: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_post_bookmarks_post_user ON post_bookmarks(post_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for post_bookmarks: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_post_bookmarks_user_created ON post_bookmarks(user_id, created_at DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for post_bookmarks user list: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_follows_follower_following ON follows(follower_id, following_id)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for follows: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_name ON users(name)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for users name: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_handle ON users(handle)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for users handle: %v\n", err)
	}

	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_users_followers_count ON users(followers_count DESC)").Error; err != nil {
		fmt.Printf("Warning: Could not create index for users followers_count: %v\n", err)
	}

	return nil
}

func addDatabaseConstraints(db *gorm.DB) error {
	// Add unique constraints to prevent duplicate likes/bookmarks/follows

	// Prevent duplicate friend requests
if err := db.Exec("ALTER TABLE friend_requests ADD CONSTRAINT uk_friend_requests_sender_receiver UNIQUE (sender_id, receiver_id)").Error; err != nil {
	fmt.Printf("Warning: Could not add unique constraint for friend_requests: %v\n", err)
}

// Prevent self friend requests
if err := db.Exec("ALTER TABLE friend_requests ADD CONSTRAINT ck_friend_requests_no_self CHECK (sender_id != receiver_id)").Error; err != nil {
	fmt.Printf("Warning: Could not add check constraint for friend_requests: %v\n", err)
}

// Prevent duplicate friendships
if err := db.Exec("ALTER TABLE friendships ADD CONSTRAINT uk_friendships_users UNIQUE (user1_id, user2_id)").Error; err != nil {
	fmt.Printf("Warning: Could not add unique constraint for friendships: %v\n", err)
}

// Ensure user1_id < user2_id in friendships
if err := db.Exec("ALTER TABLE friendships ADD CONSTRAINT ck_friendships_order CHECK (user1_id < user2_id)").Error; err != nil {
	fmt.Printf("Warning: Could not add check constraint for friendships order: %v\n", err)
}
	// Prevent duplicate post likes
	if err := db.Exec("ALTER TABLE post_likes ADD CONSTRAINT uk_post_likes_post_user UNIQUE (post_id, user_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for post_likes: %v\n", err)
	}

	// Prevent duplicate post bookmarks
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

	// NEW: Personal Route constraints
	// Prevent duplicate route bookmarks
	if err := db.Exec("ALTER TABLE saved_routes ADD CONSTRAINT uk_saved_routes_user_route UNIQUE (user_id, route_id)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for saved_routes: %v\n", err)
	}

	// Route waypoints must have valid order
	if err := db.Exec("ALTER TABLE route_waypoints ADD CONSTRAINT ck_route_waypoints_order_positive CHECK (`order` > 0)").Error; err != nil {
		fmt.Printf("Warning: Could not add check constraint for route_waypoints order: %v\n", err)
	}

	// Route waypoints combination must be unique per route
	if err := db.Exec("ALTER TABLE route_waypoints ADD CONSTRAINT uk_route_waypoints_route_order UNIQUE (route_id, `order`)").Error; err != nil {
		fmt.Printf("Warning: Could not add unique constraint for route_waypoints: %v\n", err)
	}

	// Shared Route constraints (existing)
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

	// Create sample personal routes for testing
	testRoutes := []models.Route{
		{
			ID:             "route-1",
			UserID:         "user-1",
			Name:           "My Weekend Route",
			Description:    "A nice route I planned for the weekend",
			TotalDistance:  45.8,
			TotalElevation: 650,
			EstimatedTime:  2700, // 45 minutes
			Difficulty:     "Medium",
			Tags:           models.StringSlice{"weekend", "scenic", "medium"},
			IsPublic:       false,
			TimesUsed:      2,
		},
		{
			ID:             "route-2",
			UserID:         "user-2",
			Name:           "Daily Commute Alternative",
			Description:    "Alternative route for my daily commute",
			TotalDistance:  15.2,
			TotalElevation: 120,
			EstimatedTime:  900, // 15 minutes
			Difficulty:     "Easy",
			Tags:           models.StringSlice{"commute", "daily", "easy"},
			IsPublic:       true,
			TimesUsed:      15,
		},
	}

	for _, route := range testRoutes {
		if err := db.Create(&route).Error; err != nil {
			fmt.Printf("Warning: Could not create test route %s: %v\n", route.Name, err)
		}
	}

	// Create sample shared routes for testing
	testSharedRoutes := []models.SharedRoute{
		{
			ID:                "shared-route-1",
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
			ID:                "shared-route-2",
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

	fmt.Println("Database seeded with test data including personal and shared routes")
	return nil
}
