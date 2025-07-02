// File: /models/post.go
package models

import (
	"time"
)

type Post struct {
	ID            string      `json:"id" gorm:"primaryKey"`
	UserID        string      `json:"user_id" gorm:"not null"`
	Title         string      `json:"title" gorm:"not null"`
	Subtitle      string      `json:"subtitle"`
	Routes        int         `json:"routes" gorm:"default:0"`
	Distance      string      `json:"distance"`
	Elevation     string      `json:"elevation"`
	ImageUrls     StringSlice `json:"image_urls" gorm:"type:json"`
	LikesCount    int         `json:"likes_count" gorm:"default:0"`
	CommentsCount int         `json:"comments_count" gorm:"default:0"`
	SharesCount   int         `json:"shares_count" gorm:"default:0"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`

	User      User           `json:"user" gorm:"foreignKey:UserID"`
	Likes     []PostLike     `json:"likes" gorm:"foreignKey:PostID"`
	Bookmarks []PostBookmark `json:"bookmarks" gorm:"foreignKey:PostID"`
	Comments  []Comment      `json:"comments" gorm:"foreignKey:PostID"`
}

type PostLike struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	PostID    string    `json:"post_id" gorm:"not null"`
	UserID    string    `json:"user_id" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`

	Post Post `json:"post" gorm:"foreignKey:PostID"`
	User User `json:"user" gorm:"foreignKey:UserID"`
}

// FeedResponse represents the enhanced feed response with pagination metadata
type FeedResponse struct {
	Posts      []PostWithInteractions `json:"posts"`
	Page       int                    `json:"page"`
	Limit      int                    `json:"limit"`
	Total      int64                  `json:"total"`
	HasMore    bool                   `json:"has_more"`
	TotalPages int                    `json:"total_pages"`
}
