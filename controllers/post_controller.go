// File: /controllers/post_controller.go
package controllers

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gorm.io/gorm"
	"io"
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
	minioClient            *minio.Client
	bucketName             string
}

func NewPostController(db *gorm.DB, notificationController *NotificationController) *PostController {
	// MinIO konfiguráció környezeti változókból
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	useSSL := os.Getenv("MINIO_USE_SSL") == "true"
	bucketName := os.Getenv("MINIO_BUCKET_NAME")
	
	if bucketName == "" {
		bucketName = "motocosmos-posts" // default bucket név
	}

	// MinIO client létrehozása
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to create MinIO client: %v", err))
	}

	// Bucket létrehozása, ha nem létezik
	ctx := context.Background()
	exists, err := minioClient.BucketExists(ctx, bucketName)
	if err != nil {
		panic(fmt.Sprintf("Failed to check bucket: %v", err))
	}
	
	if !exists {
		err = minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			panic(fmt.Sprintf("Failed to create bucket: %v", err))
		}
		fmt.Printf("Bucket '%s' created successfully\n", bucketName)
		
		// Bucket policy beállítása (opcionális - public read access)
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}]
		}`, bucketName)
		
		err = minioClient.SetBucketPolicy(ctx, bucketName, policy)
		if err != nil {
			fmt.Printf("Warning: Failed to set bucket policy: %v\n", err)
		}
	}

	return &PostController{
		db:                     db,
		notificationController: notificationController,
		minioClient:            minioClient,
		bucketName:             bucketName,
	}
}

// UploadImage handles image upload for posts to MinIO
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
	
	// MinIO object path: posts/{userID}/{filename}
	objectName := fmt.Sprintf("posts/%s/%s", userID, filename)
	fmt.Printf("[DEBUG] MinIO object name: %s\n", objectName)

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		fmt.Printf("[ERROR] Failed to open file: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	// Read file into buffer
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, src); err != nil {
		fmt.Printf("[ERROR] Failed to read file: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	// Determine content type
	contentType := file.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to MinIO
	ctx := context.Background()
	info, err := pc.minioClient.PutObject(
		ctx,
		pc.bucketName,
		objectName,
		bytes.NewReader(buffer.Bytes()),
		int64(buffer.Len()),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		fmt.Printf("[ERROR] Failed to upload to MinIO: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file"})
		return
	}

	fmt.Printf("[SUCCESS] File uploaded to MinIO: %s (size: %d bytes)\n", objectName, info.Size)

	// Generate URL for the uploaded image
	// Format: /api/images/{bucketName}/{objectName}
	relativePath := fmt.Sprintf("%s/%s", userID, filename)
	imageURL := fmt.Sprintf("/api/v1/posts/images/%s", relativePath)

	c.JSON(http.StatusOK, gin.H{
		"url":      imageURL,
		"filename": filename,
		"size":     file.Size,
		"message":  "Image uploaded successfully",
	})
}

func (pc *PostController) GetImage(c *gin.Context) {
    userID := c.Param("user_id")
    file := c.Param("file")
    objectName := fmt.Sprintf("posts/%s/%s", userID, file)

    obj, err := pc.minioClient.GetObject(context.Background(), pc.bucketName, objectName, minio.GetObjectOptions{})
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch file"})
        return
    }
    defer obj.Close()

    // Ellenőrizzük, hogy olvasható-e az objektum
    stat, err := obj.Stat()
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
        return
    }

    // Beállítjuk a Content-Type fejlécet az objektum ContentType-jára vagy fájlkiterjesztés alapján
    contentType := stat.ContentType
    if contentType == "" || contentType == "application/octet-stream" {
        ext := strings.ToLower(filepath.Ext(file))
        switch ext {
        case ".jpg", ".jpeg":
            contentType = "image/jpeg"
        case ".png":
            contentType = "image/png"
        case ".webp":
            contentType = "image/webp"
        default:
            contentType = "application/octet-stream"
        }
    }

    c.Header("Content-Type", contentType)

    // A letöltés helyett közvetlenül az íróra másoljuk az objektum tartalmát
    if _, err := io.Copy(c.Writer, obj); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
        return
    }
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

	uploadedImages := []map[string]interface{}{}
	ctx := context.Background()

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
		objectName := fmt.Sprintf("posts/%s/%s", userID, filename)

		// Open file
		src, err := file.Open()
		if err != nil {
			continue
		}

		// Read into buffer
		buffer := bytes.NewBuffer(nil)
		if _, err := io.Copy(buffer, src); err != nil {
			src.Close()
			continue
		}
		src.Close()

		// Determine content type
		contentType := file.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Upload to MinIO
		info, err := pc.minioClient.PutObject(
			ctx,
			pc.bucketName,
			objectName,
			bytes.NewReader(buffer.Bytes()),
			int64(buffer.Len()),
			minio.PutObjectOptions{
				ContentType: contentType,
			},
		)
		if err != nil {
			continue
		}

		// Generate URL
		imageURL := fmt.Sprintf("/api/images/%s/%s", pc.bucketName, objectName)

		uploadedImages = append(uploadedImages, map[string]interface{}{
			"url":      imageURL,
			"filename": filename,
			"size":     info.Size,
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

// DeleteImage handles image deletion from MinIO
func (pc *PostController) DeleteImage(c *gin.Context) {
	userID := c.GetString("user_id")
	imageURL := c.Query("url")

	if imageURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Image URL is required"})
		return
	}

	// Parse the URL to get the object name
	// Expected format: /api/images/{bucketName}/posts/{user_id}/{filename}
	parts := strings.Split(imageURL, "/")
	if len(parts) < 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid image URL"})
		return
	}

	// Extract: posts/{user_id}/{filename}
	urlUserID := parts[len(parts)-2]
	filename := parts[len(parts)-1]

	// Security check: ensure user can only delete their own images
	if urlUserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You can only delete your own images"})
		return
	}

	objectName := fmt.Sprintf("posts/%s/%s", urlUserID, filename)

	// Delete from MinIO
	ctx := context.Background()
	err := pc.minioClient.RemoveObject(ctx, pc.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		fmt.Printf("[ERROR] Failed to delete from MinIO: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete image"})
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
