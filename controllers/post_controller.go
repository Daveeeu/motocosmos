// File: /controllers/post_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math"
	"motocosmos-api/models"
	"net/http"
	"strconv"
)

type PostController struct {
	db                     *gorm.DB
	notificationController *NotificationController
}

func NewPostController(db *gorm.DB, notificationController *NotificationController) *PostController {
	return &PostController{
		db:                     db,
		notificationController: notificationController,
	}
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
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var posts []models.Post
	var total int64

	// Get total count
	pc.db.Model(&models.Post{}).Count(&total)

	if err := pc.db.Preload("User").Order("created_at DESC").Offset(offset).Limit(limit).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch posts"})
		return
	}

	// Convert posts to PostWithInteractions
	postsWithInteractions := make([]models.PostWithInteractions, 0, len(posts))
	for _, post := range posts {
		postWithInteractions := models.PostWithInteractions{
			Post:             post,
			UserInteractions: pc.getUserInteractions(userID, post.ID, post.UserID),
		}
		// Remove password from user data
		postWithInteractions.Post.User.Password = ""
		postsWithInteractions = append(postsWithInteractions, postWithInteractions)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.FeedResponse{
		Posts:      postsWithInteractions,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
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
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var post models.Post
	if err := pc.db.Preload("User").First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	postWithInteractions := models.PostWithInteractions{
		Post:             post,
		UserInteractions: pc.getUserInteractions(userID, post.ID, post.UserID),
	}
	postWithInteractions.Post.User.Password = ""

	c.JSON(http.StatusOK, postWithInteractions)
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

	// Delete likes and bookmarks first
	pc.db.Where("post_id = ?", postID).Delete(&models.PostLike{})
	pc.db.Where("post_id = ?", postID).Delete(&models.PostBookmark{})

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

	// Create notification for post like
	if err := pc.notificationController.CreateLikeNotification(userID, post.UserID, postID); err != nil {
		// Log error but don't fail the request
		// You might want to add proper logging here
	}

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
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var post models.Post
	if err := pc.db.First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Update shares count
	pc.db.Model(&post).UpdateColumn("shares_count", gorm.Expr("shares_count + ?", 1))

	// Create notification for post share
	if err := pc.notificationController.CreateShareNotification(userID, post.UserID, postID); err != nil {
		// Log error but don't fail the request
		// You might want to add proper logging here
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post shared successfully"})
}

// Bookmark endpoints - no notifications needed as these are private actions
func (pc *PostController) BookmarkPost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	// Check if post exists
	var post models.Post
	if err := pc.db.First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	// Check if already bookmarked
	var existingBookmark models.PostBookmark
	if err := pc.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&existingBookmark).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Post already bookmarked"})
		return
	}

	// Create bookmark
	bookmark := models.PostBookmark{
		PostID: postID,
		UserID: userID,
	}

	if err := pc.db.Create(&bookmark).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bookmark post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post bookmarked successfully"})
}

func (pc *PostController) UnbookmarkPost(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	var bookmark models.PostBookmark
	if err := pc.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&bookmark).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bookmark not found"})
		return
	}

	if err := pc.db.Delete(&bookmark).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove bookmark"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bookmark removed successfully"})
}

func (pc *PostController) GetBookmarkedPosts(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var bookmarks []models.PostBookmark
	var total int64

	// Get total count
	pc.db.Model(&models.PostBookmark{}).Where("user_id = ?", userID).Count(&total)

	if err := pc.db.Preload("Post.User").Where("user_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&bookmarks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookmarked posts"})
		return
	}

	// Convert to PostWithInteractions
	postsWithInteractions := make([]models.PostWithInteractions, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		postWithInteractions := models.PostWithInteractions{
			Post:             bookmark.Post,
			UserInteractions: pc.getUserInteractions(userID, bookmark.Post.ID, bookmark.Post.UserID),
		}
		postWithInteractions.Post.User.Password = ""
		postsWithInteractions = append(postsWithInteractions, postWithInteractions)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.FeedResponse{
		Posts:      postsWithInteractions,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

func (pc *PostController) GetFeed(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	// Get total count for all posts
	var total int64
	pc.db.Model(&models.Post{}).Count(&total)

	// Get all posts, but prioritize followed users' posts and own posts
	var posts []models.Post
	if err := pc.db.Preload("User").
		Select("posts.*, CASE WHEN (posts.user_id = ? OR posts.user_id IN (SELECT following_id FROM follows WHERE follower_id = ?)) THEN 0 ELSE 1 END as priority", userID, userID).
		Order("priority ASC, posts.created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch feed"})
		return
	}

	// Convert to PostWithInteractions
	postsWithInteractions := make([]models.PostWithInteractions, 0, len(posts))
	for _, post := range posts {
		postWithInteractions := models.PostWithInteractions{
			Post:             post,
			UserInteractions: pc.getUserInteractions(userID, post.ID, post.UserID),
		}
		// Remove password from user data
		postWithInteractions.Post.User.Password = ""
		postsWithInteractions = append(postsWithInteractions, postWithInteractions)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.FeedResponse{
		Posts:      postsWithInteractions,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

func (pc *PostController) GetPostInteractions(c *gin.Context) {
	userID := c.GetString("user_id")
	postID := c.Param("id")

	// Check if post exists
	var post models.Post
	if err := pc.db.First(&post, "id = ?", postID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Post not found"})
		return
	}

	interactions := pc.getUserInteractions(userID, postID, post.UserID)
	c.JSON(http.StatusOK, interactions)
}

// Helper function to get user interaction states
func (pc *PostController) getUserInteractions(userID, postID, postUserID string) models.UserInteractions {
	var interactions models.UserInteractions

	// Check if liked
	var like models.PostLike
	if err := pc.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&like).Error; err == nil {
		interactions.IsLiked = true
	}

	// Check if bookmarked
	var bookmark models.PostBookmark
	if err := pc.db.Where("post_id = ? AND user_id = ?", postID, userID).First(&bookmark).Error; err == nil {
		interactions.IsBookmarked = true
	}

	// Check if following the post author (skip if it's the user's own post)
	if postUserID != userID {
		var follow models.Follow
		if err := pc.db.Where("follower_id = ? AND following_id = ?", userID, postUserID).First(&follow).Error; err == nil {
			interactions.IsUserFollowing = true
		}
	}

	return interactions
}
