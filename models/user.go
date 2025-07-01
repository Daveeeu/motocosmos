// File: /models/user.go
package models

import (
	"time"
)

type User struct {
	ID             string    `json:"id" gorm:"primaryKey;size:191"`
	Name           string    `json:"name" gorm:"not null;size:255"`
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
