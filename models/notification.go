// File: /models/notification.go
package models

import (
	"fmt"
	"time"
)

type NotificationType string

const (
	NotificationTypeFollow      NotificationType = "follow"
	NotificationTypeLike        NotificationType = "like"
	NotificationTypeComment     NotificationType = "comment"
	NotificationTypeCommentLike NotificationType = "comment_like"
	NotificationTypeShare       NotificationType = "share"
)

type Notification struct {
	ID           string           `json:"id" gorm:"primaryKey;size:191"`
	Type         NotificationType `json:"type" gorm:"not null;size:50"`
	ActorUserID  string           `json:"actor_user_id" gorm:"not null;size:191"`  // Who performed the action
	TargetUserID string           `json:"target_user_id" gorm:"not null;size:191"` // Who receives the notification
	PostID       *string          `json:"post_id" gorm:"size:191"`                 // Optional: related post
	CommentID    *string          `json:"comment_id" gorm:"size:191"`              // Optional: related comment
	IsRead       bool             `json:"is_read" gorm:"default:false"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`

	// Relationships
	ActorUser  User  `json:"actor_user" gorm:"foreignKey:ActorUserID"`
	TargetUser User  `json:"target_user" gorm:"foreignKey:TargetUserID"`
	Post       *Post `json:"post,omitempty" gorm:"foreignKey:PostID"`
}

// NotificationResponse represents the API response for notifications
type NotificationResponse struct {
	ID        string            `json:"id"`
	Type      NotificationType  `json:"type"`
	ActorUser NotificationUser  `json:"actor_user"`
	Post      *NotificationPost `json:"post,omitempty"`
	IsRead    bool              `json:"is_read"`
	CreatedAt time.Time         `json:"created_at"`
	Message   string            `json:"message"`
	TimeAgo   string            `json:"time_ago"`
}

type NotificationUser struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Handle string  `json:"handle"`
	Avatar *string `json:"avatar"`
}

type NotificationPost struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	ImageURL *string `json:"image_url,omitempty"`
}

// NotificationStats represents notification statistics
type NotificationStats struct {
	UnreadCount int `json:"unread_count"`
	TotalCount  int `json:"total_count"`
}

// PaginatedNotifications represents paginated notification response
type PaginatedNotifications struct {
	Notifications []NotificationResponse `json:"notifications"`
	Page          int                    `json:"page"`
	Limit         int                    `json:"limit"`
	Total         int64                  `json:"total"`
	HasMore       bool                   `json:"has_more"`
	TotalPages    int                    `json:"total_pages"`
}

// CreateNotificationParams for creating new notifications
type CreateNotificationParams struct {
	Type         NotificationType `json:"type"`
	ActorUserID  string           `json:"actor_user_id"`
	TargetUserID string           `json:"target_user_id"`
	PostID       *string          `json:"post_id,omitempty"`
	CommentID    *string          `json:"comment_id,omitempty"`
}

// GetNotificationMessage returns a human-readable message for the notification
func (n *Notification) GetNotificationMessage() string {
	switch n.Type {
	case NotificationTypeFollow:
		return "started following you"
	case NotificationTypeLike:
		return "liked your post"
	case NotificationTypeComment:
		return "commented on your post"
	case NotificationTypeCommentLike:
		return "liked your comment"
	case NotificationTypeShare:
		return "shared your post"
	default:
		return "interacted with your content"
	}
}

// GetTimeAgo returns a human-readable time difference
func (n *Notification) GetTimeAgo() string {
	now := time.Now()
	diff := now.Sub(n.CreatedAt)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		minutes := int(diff.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case diff < 24*time.Hour:
		hours := int(diff.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case diff < 7*24*time.Hour:
		days := int(diff.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case diff < 30*24*time.Hour:
		weeks := int(diff.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	default:
		months := int(diff.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	}
}

// ToResponse converts Notification to NotificationResponse
func (n *Notification) ToResponse() NotificationResponse {
	response := NotificationResponse{
		ID:        n.ID,
		Type:      n.Type,
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
		Message:   n.GetNotificationMessage(),
		TimeAgo:   n.GetTimeAgo(),
		ActorUser: NotificationUser{
			ID:     n.ActorUser.ID,
			Name:   n.ActorUser.Name,
			Handle: n.ActorUser.Handle,
			Avatar: n.ActorUser.Avatar,
		},
	}

	// Add post information if present
	if n.Post != nil {
		imageURL := ""
		if n.Post.ImageUrls != nil && len(n.Post.ImageUrls) > 0 {
			imageURL = n.Post.ImageUrls[0]
		}
		response.Post = &NotificationPost{
			ID:       n.Post.ID,
			Title:    n.Post.Title,
			ImageURL: &imageURL,
		}
	}

	return response
}
