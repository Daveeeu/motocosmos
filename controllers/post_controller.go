// File: /controllers/post_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"strconv"
)

type PostController struct {
	db *gorm.DB
}

func NewPostController(db *gorm.DB) *PostController {
	return &PostController{db: db}
}

type CreatePostRequest struct {
	Title     string   `json:"title" binding:"required"`
	Subtitle  string   `json:"subtitle"`
	Routes    int      `json:"routes"`
	Distance  string   `json:"distance"`
	Elevation string   `json:"elevation"`
	ImageUrls []string `json:"image_urls"`
}

func (pc *PostController) GetPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var posts []models.Post
	if err := pc.db.Preload("User").Order("created_at DESC").Offset(offset).Limit(limit).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
	}

	// Remove password from user data
	for i := range posts {
		posts[i].User.Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"page":  page,
		"limit": limit,
	})
}

func (pc *PostController) CreatePost(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post := models.Post{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     req.Title,
		Subtitle:  req.Subtitle,
		Routes:    req.Routes,
		Distance:  req.Distance,
		Elevation: req.Elevation,
		ImageUrls: models.StringSlice(req.ImageUrls),
	}

	if err := pc.db.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	// Load the complete post with user info
	pc.db.Preload("User").First(&post, "id = ?", post.ID)
	post.User.Password = ""

	c.JSON(http.StatusCreated, post)
}

func (pc *PostController) GetPost(c *gin.Context) {
	postID := c.Param("id")

	var post models.Post
	if err := pc.db.Preload("User").First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	post.User.Password = ""
	c.JSON(http.StatusOK, post)
}

func (pc *PostController) UpdatePost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var post models.Post
	if err := pc.db.First(&post, "id = ? AND user_id = ?", postID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found or access denied"})
		return
	}

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"title":      req.Title,
		"subtitle":   req.Subtitle,
		"routes":     req.Routes,
		"distance":   req.Distance,
		"elevation":  req.Elevation,
		"image_urls": models.StringSlice(req.ImageUrls),
	}

	if err := pc.db.Model(&post).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post updated successfully"})
}

func (pc *PostController) DeletePost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var post models.Post
	if err := pc.db.First(&post, "id = ? AND user_id = ?", postID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found or access denied"})
		return
	}

	// Delete likes first
	pc.db.Where("post_id = ?", postID).Delete(&models.PostLike{})

	// Delete the post
	if err := pc.db.Delete(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

func (pc *PostController) LikePost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	// Check if post exists
	var post models.Post
	if err := pc.db.First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Check if already liked
	var existingLike models.PostLike
	if err := pc.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&existingLike).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Post already liked"})
		return
	}

	// Create like
	like := models.PostLike{
		PostID: postID,
		UserID: userID,
	}

	if err := pc.db.Create(&like).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like post"})
		return
	}

	// Update likes count
	pc.db.Model(&post).UpdateColumn("likes_count", gorm.Expr("likes_count + ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Post liked successfully"})
}

func (pc *PostController) UnlikePost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var like models.PostLike
	if err := pc.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Like not found"})
		return
	}

	if err := pc.db.Delete(&like).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlike post"})
		return
	}

	// Update likes count
	pc.db.Model(&models.Post{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count - ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Post unliked successfully"})
}

func (pc *PostController) SharePost(c *gin.Context) {
	postID := c.Param("id")

	var post models.Post
	if err := pc.db.First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Update shares count
	pc.db.Model(&post).UpdateColumn("shares_count", gorm.Expr("shares_count + ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Post shared successfully"})
}

func (pc *PostController) GetFeed(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	// Get posts from followed users and own posts
	var posts []models.Post
	if err := pc.db.Preload("User").Where(`
        user_id = ? OR user_id IN (
            SELECT following_id FROM follows WHERE follower_id = ?
        )
    `, userID, userID).Order("created_at DESC").Offset(offset).Limit(limit).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feed"})
		return
	}

	// Remove password from user data
	for i := range posts {
		posts[i].User.Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"page":  page,
		"limit": limit,
	})
}
