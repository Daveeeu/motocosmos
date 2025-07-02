package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"time"
)

type CommentController struct {
	db                     *gorm.DB
	notificationController *NotificationController
}

func NewCommentController(db *gorm.DB, notificationController *NotificationController) *CommentController {
	return &CommentController{
		db:                     db,
		notificationController: notificationController,
	}
}

type CreateCommentRequest struct {
	Body string `json:"body" binding:"required"`
}

func (cc *CommentController) CreateComment(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment := models.Comment{
		ID:        uuid.New().String(),
		PostID:    postID,
		UserID:    userID,
		Body:      req.Body,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := cc.db.Create(&comment).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create comment"})
		return
	}

	// Növeld a post comments_count mezőjét
	cc.db.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("comments_count", gorm.Expr("comments_count + ?", 1))

	var post models.Post
	if err := cc.db.First(&post, "id = ?", postID).Error; err == nil {
		// Ne küldj értesítést, ha valaki a saját posztjára kommentel
		if post.UserID != userID {
			if err := cc.notificationController.CreateCommentNotification(
				userID,      // aki kommentelt
				post.UserID, // poszt szerzője (értesítendő)
				postID,
			); err != nil {
				// Logold az esetleges hibát, de ne állítsd meg a folyamatot
				fmt.Printf("Failed to create comment notification: %v\n", err)
			}
		}
	}

	c.JSON(http.StatusCreated, comment)
}

func (cc *CommentController) GetComments(c *gin.Context) {
	postID := c.Param("id")
	var comments []models.Comment
	if err := cc.db.Preload("User").Where("post_id = ?", postID).Order("created_at ASC").Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch comments"})
		return
	}
	c.JSON(http.StatusOK, comments)
}
