// File: /models/shared_route.go
package models

import (
	"time"
)

// SharedRoutePoint represents a point in a shared route with optional elevation
type SharedRoutePoint struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Elevation *float64 `json:"elevation,omitempty"`
}

// SharedRoute represents a publicly shared route that users can explore
type SharedRoute struct {
	ID                string      `json:"id" gorm:"primaryKey;size:191"`
	Title             string      `json:"title" gorm:"not null;size:255"`
	Description       string      `json:"description" gorm:"type:text"`
	CreatorID         string      `json:"creator_id" gorm:"not null;size:191"`
	CreatorName       string      `json:"creator_name" gorm:"not null;size:255"`
	CreatorAvatar     string      `json:"creator_avatar" gorm:"size:255"`
	ImageUrls         StringSlice `json:"image_urls" gorm:"type:json"`
	RoutePoints       JSONData    `json:"route_points" gorm:"type:json"` // Array of SharedRoutePoint
	TotalDistance     float64     `json:"total_distance"`                // km
	TotalElevation    float64     `json:"total_elevation"`               // m
	EstimatedDuration int         `json:"estimated_duration"`            // seconds
	Difficulty        string      `json:"difficulty" gorm:"size:50"`     // Easy, Medium, Hard
	Tags              StringSlice `json:"tags" gorm:"type:json"`
	LikesCount        int         `json:"likes_count" gorm:"default:0"`
	CommentsCount     int         `json:"comments_count" gorm:"default:0"`
	DownloadsCount    int         `json:"downloads_count" gorm:"default:0"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`

	// Relationships
	Creator   User                  `json:"creator" gorm:"foreignKey:CreatorID"`
	Likes     []SharedRouteLike     `json:"likes" gorm:"foreignKey:RouteID"`
	Bookmarks []SharedRouteBookmark `json:"bookmarks" gorm:"foreignKey:RouteID"`
}

// SharedRouteLike represents a like on a shared route
type SharedRouteLike struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	RouteID   string    `json:"route_id" gorm:"not null;size:191"`
	UserID    string    `json:"user_id" gorm:"not null;size:191"`
	CreatedAt time.Time `json:"created_at"`

	Route SharedRoute `json:"route" gorm:"foreignKey:RouteID"`
	User  User        `json:"user" gorm:"foreignKey:UserID"`
}

// SharedRouteBookmark represents a bookmark on a shared route
type SharedRouteBookmark struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	RouteID   string    `json:"route_id" gorm:"not null;size:191"`
	UserID    string    `json:"user_id" gorm:"not null;size:191"`
	CreatedAt time.Time `json:"created_at"`

	Route SharedRoute `json:"route" gorm:"foreignKey:RouteID"`
	User  User        `json:"user" gorm:"foreignKey:UserID"`
}

// SharedRouteWithInteractions represents a shared route with user interaction states
type SharedRouteWithInteractions struct {
	SharedRoute
	UserInteractions SharedRouteInteractions `json:"user_interactions"`
}

// SharedRouteInteractions represents the current user's interactions with a shared route
type SharedRouteInteractions struct {
	IsLiked      bool `json:"is_liked"`
	IsBookmarked bool `json:"is_bookmarked"`
}

// SharedRouteResponse represents the API response for shared routes with pagination
type SharedRouteResponse struct {
	Routes     []SharedRouteWithInteractions `json:"routes"`
	Page       int                           `json:"page"`
	Limit      int                           `json:"limit"`
	Total      int64                         `json:"total"`
	HasMore    bool                          `json:"has_more"`
	TotalPages int                           `json:"total_pages"`
}

// PopularTagsResponse represents popular tags response
type PopularTagsResponse struct {
	Tags []TagInfo `json:"tags"`
}

// TagInfo represents tag information with count
type TagInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SharedRouteStats represents statistics for a shared route
type SharedRouteStats struct {
	TotalRoutes    int64     `json:"total_routes"`
	TotalDistance  float64   `json:"total_distance"`
	TotalDownloads int64     `json:"total_downloads"`
	PopularTags    []TagInfo `json:"popular_tags"`
}
