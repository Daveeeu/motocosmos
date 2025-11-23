// File: /models/location.go
package models

import (
	"time"
)

// UserLocation stores the current location of a user
type UserLocation struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"user_id" gorm:"not null;uniqueIndex"`
	Username         string    `json:"username" gorm:"not null"`
	Latitude         float64   `json:"latitude" gorm:"not null"`
	Longitude        float64   `json:"longitude" gorm:"not null"`
	Accuracy         float64   `json:"accuracy"` // Accuracy in meters
	IsAvailable      bool      `json:"is_available" gorm:"default:true"`
	IsOnline         bool      `json:"is_online" gorm:"default:false"`
	LastSeen         time.Time `json:"last_seen"`
	Status           string    `json:"status" gorm:"size:255"`
	IsLocationPublic bool      `json:"is_location_public" gorm:"default:false"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}

// LocationVisibilitySettings stores user's location sharing preferences
type LocationVisibilitySettings struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	UserID         string    `json:"user_id" gorm:"not null;uniqueIndex"`
	VisibilityMode string    `json:"visibility_mode" gorm:"not null;default:'friends'"` // all, friends, custom, none
	AccuracyLevel  string    `json:"accuracy_level" gorm:"not null;default:'precise'"`  // precise, approximate, city
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	User          User                        `json:"user" gorm:"foreignKey:UserID"`
	AllowedUsers  []LocationVisibilityAllowed `json:"allowed_users" gorm:"foreignKey:OwnerUserID"`
}

// LocationVisibilityAllowed stores custom visibility permissions
type LocationVisibilityAllowed struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	OwnerUserID  string    `json:"owner_user_id" gorm:"not null;index:idx_location_visibility"`
	AllowedUserID string   `json:"allowed_user_id" gorm:"not null;index:idx_location_visibility"`
	CreatedAt    time.Time `json:"created_at"`

	OwnerUser   User `json:"owner_user" gorm:"foreignKey:OwnerUserID"`
	AllowedUser User `json:"allowed_user" gorm:"foreignKey:AllowedUserID"`
}

// TableName overrides
func (UserLocation) TableName() string {
	return "user_locations"
}

func (LocationVisibilitySettings) TableName() string {
	return "location_visibility_settings"
}

func (LocationVisibilityAllowed) TableName() string {
	return "location_visibility_allowed"
}

// DTO Models for API responses

// LocatorUserResponse represents a user on the map
type LocatorUserResponse struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	AvatarURL    string    `json:"avatar_url"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	IsAvailable  bool      `json:"is_available"`
	LastSeen     time.Time `json:"last_seen"`
	DistanceKm   float64   `json:"distance_km,omitempty"`
	Status       string    `json:"status,omitempty"`
}

// LocatorResponse is the main response for GET /locator
type LocatorResponse struct {
	Count int                   `json:"count"`
	Users []LocatorUserResponse `json:"users"`
}

// UpdateLocationRequest for POST /locator/location
type UpdateLocationRequest struct {
	Latitude      float64 `json:"latitude" binding:"required"`
	Longitude     float64 `json:"longitude" binding:"required"`
	Accuracy      float64 `json:"accuracy"`
	IsAvailable   *bool   `json:"is_available"`
	AccuracyLevel string  `json:"accuracy_level"` // precise, approximate, city
	Status        string  `json:"status"`
}

// UpdateVisibilityRequest for POST /locator/visibility
type UpdateVisibilityRequest struct {
	VisibilityMode string   `json:"visibility_mode" binding:"required,oneof=all friends custom none"`
	AccuracyLevel  string   `json:"accuracy_level" binding:"required,oneof=precise approximate city"`
	AllowedUserIDs []string `json:"allowed_user_ids"`
}

// VisibilitySettingsResponse for GET /locator/settings
type VisibilitySettingsResponse struct {
	VisibilityMode string                  `json:"visibility_mode"`
	AccuracyLevel  string                  `json:"accuracy_level"`
	AllowedUsers   []AllowedUserResponse   `json:"allowed_users"`
}

// AllowedUserResponse represents a user in the allowed list
type AllowedUserResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}