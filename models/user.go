// File: /models/user.go
package models

import (
	"strings"
	"time"
)

type User struct {
	ID             string    `json:"id" gorm:"primaryKey;size:191"`
	Name           string    `json:"name" gorm:"not null;size:255"`
	Handle         string    `json:"handle" gorm:"uniqueIndex;not null;size:50"` // Added for @username functionality
	Email          string    `json:"email" gorm:"uniqueIndex;not null;size:255"`
	Password       string    `json:"-" gorm:"not null;size:255"`
	EmailVerified  bool      `json:"email_verified" gorm:"default:false"`
	Avatar         *string   `json:"avatar" gorm:"size:500"`
	FollowersCount int       `json:"followers_count" gorm:"default:0"`
	FollowingCount int       `json:"following_count" gorm:"default:0"`
	RidesCount     int       `json:"rides_count" gorm:"default:0"`
	TotalTime      string    `json:"total_time" gorm:"default:'0h 0m';size:50"`
	TotalDistance  string    `json:"total_distance" gorm:"default:'0 km';size:50"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	// Relationships
	Motorcycles   []Motorcycle     `json:"motorcycles" gorm:"foreignKey:UserID"`
	Posts         []Post           `json:"posts" gorm:"foreignKey:UserID"`
	CreatedEvents []CommunityEvent `json:"created_events" gorm:"foreignKey:OrganizerID"`
	RideRecords   []RideRecord     `json:"ride_records" gorm:"foreignKey:UserID"`
}

type Follow struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	FollowerID  string    `json:"follower_id" gorm:"not null;size:191"`
	FollowingID string    `json:"following_id" gorm:"not null;size:191"`
	CreatedAt   time.Time `json:"created_at"`

	Follower  User `json:"follower" gorm:"foreignKey:FollowerID"`
	Following User `json:"following" gorm:"foreignKey:FollowingID"`
}

// PostBookmark represents a bookmarked post by a user
type PostBookmark struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    string    `json:"post_id" gorm:"not null;size:191"`
	UserID    string    `json:"user_id" gorm:"not null;size:191"`
	CreatedAt time.Time `json:"created_at"`

	Post Post `json:"post" gorm:"foreignKey:PostID"`
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// GenerateHandleFromName creates a unique handle from the user's name
func GenerateHandleFromName(name string) string {
	// Convert to lowercase and replace spaces with underscores
	handle := strings.ToLower(strings.ReplaceAll(name, " ", "_"))
	// Remove special characters
	handle = strings.ReplaceAll(handle, ".", "")
	handle = strings.ReplaceAll(handle, "-", "_")
	return handle
}

// UserInteractions represents the current user's interactions with posts
type UserInteractions struct {
	IsLiked         bool `json:"is_liked"`
	IsBookmarked    bool `json:"is_bookmarked"`
	IsUserFollowing bool `json:"is_user_following"`
}

// PostWithInteractions represents a post with user interaction states
type PostWithInteractions struct {
	Post
	UserInteractions UserInteractions `json:"user_interactions"`
}
