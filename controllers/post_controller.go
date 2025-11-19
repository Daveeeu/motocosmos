// File: /controllers/post_controller.go
package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math"
	"mime/multipart"
	"motocosmos-api/models"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PostController struct {
	db                     *gorm.DB
	notificationController *NotificationController
	uploadPath             string // Path where images will be stored
}

func NewPostController(db *gorm.DB, notificationController *NotificationController) *PostController {
	// Create uploads directory if it doesn't exist
	uploadPath := "./uploads/posts"
	if err := os.MkdirAll(uploadPath, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create upload directory: %v", err))
	}

	return &PostController{
		db:                     db,
		notificationController: notificationController,
		uploadPath:             uploadPath,
	}
}

// UploadImage handles image upload for posts
func (pc *PostController) UploadImage(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse multipart form
	file, err := c.FormFile("image")
	if err != nil {
		fmt.Printf("[ERROR] Failed to get file: %v\n", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image file provided"})
		return
	}

	fmt.Printf("[DEBUG] Received file: %s, size: %d\n", file.Filename, file.Size)

	// Validate file size (max 10MB)
	if file.Size > 10*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File size too large (max 10MB)"})
		return
	}

	// Validate file type
	if !isValidImageType(file) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPG, PNG, and WebP are allowed"})
		return
	}

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	fmt.Printf("[DEBUG] Generated filename: %s\n", filename)

	// Create user-specific subdirectory
	userUploadPath := filepath.Join(pc.uploadPath, userID)
	fmt.Printf("[DEBUG] User upload path: %s\n", userUploadPath)
	
	if err := os.MkdirAll(userUploadPath, 0755); err != nil {
		fmt.Printf("[ERROR] Failed to create directory: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Full path for the file
	filePath := filepath.Join(userUploadPath, filename)
	fmt.Printf("[DEBUG] Full file path: %s\n", filePath)

	// Save the file
	if err := c.SaveUploadedFile(file, filePath); err != nil {
		fmt.Printf("[ERROR] Failed to save file: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Printf("[ERROR] File does not exist after save: %s\n", filePath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "File was not saved"})
		return
	}

	fileInfo, _ := os.Stat(filePath)
	fmt.Printf("[SUCCESS] File saved successfully: %s (size: %d bytes)\n", filePath, fileInfo.Size())

	// Generate URL for the uploaded image
	imageURL := fmt.Sprintf("/uploads/posts/%s/%s", userID, filename)

	c.JSON(http.StatusOK, gin.H{
		"url":      imageURL,
		"filename": filename,
		"size":     file.Size,
		"message":  "Image uploaded successfully",
	})
}

// UploadMultipleImages handles multiple image uploads at once
func (pc *PostController) UploadMultipleImages(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No images provided"})
		return
	}

	// Limit to 10 images per request
	if len(files) > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Too many images (max 10)"})
		return
	}

	// Create user-specific subdirectory
	userUploadPath := filepath.Join(pc.uploadPath, userID)
	if err := os.MkdirAll(userUploadPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	uploadedImages := []map[string]interface{}{}

	for _, file := range files {
		// Validate file size
		if file.Size > 10*1024*1024 {
			continue // Skip files larger than 10MB
		}

		// Validate file type
		if !isValidImageType(file) {
			continue // Skip invalid file types
		}

		// Generate unique filename
		ext := filepath.Ext(file.Filename)
		filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
		filepath := filepath.Join(userUploadPath, filename)

		// Save the file
		if err := c.SaveUploadedFile(file, filepath); err != nil {
			continue // Skip files that fail to save
		}

		// Generate URL
		imageURL := fmt.Sprintf("/uploads/posts/%s/%s", userID, filename)

		uploadedImages = append(uploadedImages, map[string]interface{}{
			"url":      imageURL,
			"filename": filename,
			"size":     file.Size,
		})
	}

	if len(uploadedImages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid images were uploaded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"images":  uploadedImages,
		"count":   len(uploadedImages),
		"message": "Images uploaded successfully",
	})
}

// DeleteImage handles image deletion
func (pc *PostController) DeleteImage(c *gin.Context) {
	userID := c.GetString("user_id")
	imageURL := c.Query("url")

	if imageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image URL is required"})
		return
	}

	// Parse the URL to get the filename
	// Expected format: /uploads/posts/{user_id}/{filename}
	parts := strings.Split(imageURL, "/")
	if len(parts) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image URL"})
		return
	}

	urlUserID := parts[len(parts)-2]
	filename := parts[len(parts)-1]

	// Security check: ensure user can only delete their own images
	if urlUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own images"})
		return
	}

	// Build file path
	filePath := filepath.Join(pc.uploadPath, urlUserID, filename)

	// Delete the file
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Image not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Image deleted successfully"})
}

// Helper function to validate image type
func isValidImageType(file *multipart.FileHeader) bool {
	// Check file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	validExtensions := []string{".jpg", ".jpeg", ".png", ".webp", ".gif"}
	
	for _, validExt := range validExtensions {
		if ext == validExt {
			return true
		}
	}

	// Also check MIME type if available
	if file.Header != nil {
		contentType := file.Header.Get("Content-Type")
		validTypes := []string{"image/jpeg", "image/png", "image/webp", "image/gif"}
		
		for _, validType := range validTypes {
			if contentType == validType {
				return true
			}
		}
	}

	return false
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
