// File: /controllers/notification_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math"
	"motocosmos-api/models"
	"net/http"
	"strconv"
	"time"
)

type NotificationController struct {
	db *gorm.DB
}

func NewNotificationController(db *gorm.DB) *NotificationController {
	return &NotificationController{db: db}
}

// GetNotifications gets paginated notifications for the current user
func (nc *NotificationController) GetNotifications(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	notificationType := c.Query("type") // Optional filter by type

	// Validate page and limit
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	offset := (page - 1) * limit

	// Build query
	query := nc.db.Where("target_user_id = ?", userID)

	// Add type filter if specified
	if notificationType != "" {
		query = query.Where("type = ?", notificationType)
	}

	// Get total count
	var total int64
	query.Model(&models.Notification{}).Count(&total)

	// Get notifications with relationships
	var notifications []models.Notification
	if err := query.Preload("ActorUser").
		Preload("Post").
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&notifications).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notifications"})
		return
	}

	// Convert to response format
	var responses []models.NotificationResponse
	for _, notification := range notifications {
		responses = append(responses, notification.ToResponse())
	}

	// Calculate pagination info
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.PaginatedNotifications{
		Notifications: responses,
		Page:          page,
		Limit:         limit,
		Total:         total,
		HasMore:       hasMore,
		TotalPages:    totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetNotificationStats gets notification statistics (unread count, etc.)
func (nc *NotificationController) GetNotificationStats(c *gin.Context) {
	userID := c.GetString("user_id")

	var unreadCount int64
	var totalCount int64

	// Count unread notifications
	if err := nc.db.Model(&models.Notification{}).
		Where("target_user_id = ? AND is_read = ?", userID, false).
		Count(&unreadCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notification stats"})
		return
	}

	// Count total notifications
	if err := nc.db.Model(&models.Notification{}).
		Where("target_user_id = ?", userID).
		Count(&totalCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notification stats"})
		return
	}

	stats := models.NotificationStats{
		UnreadCount: int(unreadCount),
		TotalCount:  int(totalCount),
	}

	c.JSON(http.StatusOK, stats)
}

// MarkAsRead marks a notification as read
func (nc *NotificationController) MarkAsRead(c *gin.Context) {
	userID := c.GetString("user_id")
	notificationID := c.Param("id")

	var notification models.Notification
	if err := nc.db.Where("id = ? AND target_user_id = ?", notificationID, userID).
		First(&notification).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find notification"})
		}
		return
	}

	if err := nc.db.Model(&notification).Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

// MarkAllAsRead marks all notifications as read for the current user
func (nc *NotificationController) MarkAllAsRead(c *gin.Context) {
	userID := c.GetString("user_id")

	if err := nc.db.Model(&models.Notification{}).
		Where("target_user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notifications as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All notifications marked as read"})
}

// DeleteNotification deletes a notification
func (nc *NotificationController) DeleteNotification(c *gin.Context) {
	userID := c.GetString("user_id")
	notificationID := c.Param("id")

	var notification models.Notification
	if err := nc.db.Where("id = ? AND target_user_id = ?", notificationID, userID).
		First(&notification).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Notification not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find notification"})
		}
		return
	}

	if err := nc.db.Delete(&notification).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete notification"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification deleted successfully"})
}

// CreateNotification creates a new notification (internal use)
func (nc *NotificationController) CreateNotification(params models.CreateNotificationParams) error {
	// Don't create notification if actor and target are the same
	if params.ActorUserID == params.TargetUserID {
		return nil
	}

	// Check for duplicate notifications (within last hour for same action)
	var existingNotification models.Notification
	err := nc.db.Where("type = ? AND actor_user_id = ? AND target_user_id = ? AND post_id = ? AND created_at > ?",
		params.Type, params.ActorUserID, params.TargetUserID, params.PostID,
		time.Now().Add(-1*time.Hour)).First(&existingNotification).Error

	if err == nil {
		// Duplicate notification exists, don't create another
		return nil
	}

	notification := models.Notification{
		ID:           uuid.New().String(),
		Type:         params.Type,
		ActorUserID:  params.ActorUserID,
		TargetUserID: params.TargetUserID,
		PostID:       params.PostID,
		CommentID:    params.CommentID,
		IsRead:       false,
	}

	return nc.db.Create(&notification).Error
}

// Helper methods for creating specific notification types

// CreateFollowNotification creates a follow notification
func (nc *NotificationController) CreateFollowNotification(actorUserID, targetUserID string) error {
	return nc.CreateNotification(models.CreateNotificationParams{
		Type:         models.NotificationTypeFollow,
		ActorUserID:  actorUserID,
		TargetUserID: targetUserID,
	})
}

// CreateLikeNotification creates a like notification
func (nc *NotificationController) CreateLikeNotification(actorUserID, targetUserID, postID string) error {
	return nc.CreateNotification(models.CreateNotificationParams{
		Type:         models.NotificationTypeLike,
		ActorUserID:  actorUserID,
		TargetUserID: targetUserID,
		PostID:       &postID,
	})
}

// CreateCommentNotification creates a comment notification
func (nc *NotificationController) CreateCommentNotification(actorUserID, targetUserID, postID string) error {
	return nc.CreateNotification(models.CreateNotificationParams{
		Type:         models.NotificationTypeComment,
		ActorUserID:  actorUserID,
		TargetUserID: targetUserID,
		PostID:       &postID,
	})
}

// CreateCommentLikeNotification creates a comment like notification
func (nc *NotificationController) CreateCommentLikeNotification(actorUserID, targetUserID, postID, commentID string) error {
	return nc.CreateNotification(models.CreateNotificationParams{
		Type:         models.NotificationTypeCommentLike,
		ActorUserID:  actorUserID,
		TargetUserID: targetUserID,
		PostID:       &postID,
		CommentID:    &commentID,
	})
}

// CreateShareNotification creates a share notification
func (nc *NotificationController) CreateShareNotification(actorUserID, targetUserID, postID string) error {
	return nc.CreateNotification(models.CreateNotificationParams{
		Type:         models.NotificationTypeShare,
		ActorUserID:  actorUserID,
		TargetUserID: targetUserID,
		PostID:       &postID,
	})
}
