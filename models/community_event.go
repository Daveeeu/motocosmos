// File: /models/community_event.go
package models

import (
	"time"
)

type CommunityEvent struct {
	ID                string      `json:"id" gorm:"primaryKey;size:191"`
	Title             string      `json:"title" gorm:"not null;size:255"`
	Description       string      `json:"description" gorm:"not null;type:text"`
	OrganizerID       string      `json:"organizer_id" gorm:"not null;size:191"`
	OrganizerName     string      `json:"organizer_name" gorm:"not null;size:255"`
	OrganizerAvatar   string      `json:"organizer_avatar" gorm:"size:10"`
	EventDate         time.Time   `json:"event_date" gorm:"not null"`
	LocationName      string      `json:"location_name" gorm:"not null;size:255"`
	LocationLatitude  float64     `json:"location_latitude" gorm:"not null"`
	LocationLongitude float64     `json:"location_longitude" gorm:"not null"`
	LocationAddress   string      `json:"location_address" gorm:"size:500"`
	Difficulty        string      `json:"difficulty" gorm:"not null;size:50"`
	EstimatedDistance float64     `json:"estimated_distance"`
	EstimatedDuration int         `json:"estimated_duration"` // in seconds
	MaxParticipants   int         `json:"max_participants" gorm:"not null"`
	ParticipantsCount int         `json:"participants_count" gorm:"default:0"`
	Tags              StringSlice `json:"tags" gorm:"type:json"`
	RouteID           *string     `json:"route_id" gorm:"size:191"`
	ImageUrls         StringSlice `json:"image_urls" gorm:"type:json"`
	LikesCount        int         `json:"likes_count" gorm:"default:0"`
	IsFull            bool        `json:"is_full" gorm:"default:false"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`

	Organizer    User               `json:"organizer" gorm:"foreignKey:OrganizerID"`
	Route        *Route             `json:"route" gorm:"foreignKey:RouteID"`
	Participants []EventParticipant `json:"participants" gorm:"foreignKey:EventID"`
}

type EventParticipant struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	EventID   string    `json:"event_id" gorm:"not null;size:191"`
	UserID    string    `json:"user_id" gorm:"not null;size:191"`
	IsLiked   bool      `json:"is_liked" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`

	Event CommunityEvent `json:"event" gorm:"foreignKey:EventID"`
	User  User           `json:"user" gorm:"foreignKey:UserID"`
}
