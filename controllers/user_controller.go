// File: /controllers/user_controller.go
package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"strconv"
)

type UserController struct {
	db                     *gorm.DB
	notificationController *NotificationController
}

func NewUserController(db *gorm.DB, notificationController *NotificationController) *UserController {
	return &UserController{
		db:                     db,
		notificationController: notificationController,
	}
}

func (uc *UserController) GetProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var user models.User
	if err := uc.db.Preload("Motorcycles").First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Password = ""
	c.JSON(http.StatusOK, user)
}

func (uc *UserController) UpdateProfile(c *gin.Context) {
	userID := c.GetString("user_id")

	var req struct {
		Name   string  `json:"name"`
		Handle string  `json:"handle"`
		Avatar *string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := uc.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Handle != "" {
		// Check if handle is already taken
		var existingUser models.User
		if err := uc.db.Where("handle = ? AND id != ?", req.Handle, userID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Handle already taken"})
			return
		}
		updates["handle"] = req.Handle
	}
	if req.Avatar != nil {
		updates["avatar"] = req.Avatar
	}

	if err := uc.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (uc *UserController) GetStatistics(c *gin.Context) {
	userID := c.GetString("user_id")

	var user models.User
	if err := uc.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	stats := gin.H{
		"followers_count": user.FollowersCount,
		"following_count": user.FollowingCount,
		"rides_count":     user.RidesCount,
		"total_time":      user.TotalTime,
		"total_distance":  user.TotalDistance,
	}

	c.JSON(http.StatusOK, stats)
}

func (uc *UserController) FollowUser(c *gin.Context) {
	followerID := c.GetString("user_id")
	followingID := c.Param("user_id")

	if followerID == followingID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot follow yourself"})
		return
	}

	// Check if target user exists
	var targetUser models.User
	if err := uc.db.First(&targetUser, "id = ?", followingID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if already following
	var existingFollow models.Follow
	if err := uc.db.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&existingFollow).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already following this user"})
		return
	}

	// Create follow relationship
	follow := models.Follow{
		FollowerID:  followerID,
		FollowingID: followingID,
	}

	if err := uc.db.Create(&follow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to follow user"})
		return
	}

	// Update follower and following counts
	uc.db.Model(&models.User{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("following_count + ?", 1))
	uc.db.Model(&models.User{}).Where("id = ?", followingID).UpdateColumn("followers_count", gorm.Expr("followers_count + ?", 1))

	// Create notification for follow
	if err := uc.notificationController.CreateFollowNotification(followerID, followingID); err != nil {
		// Log error but don't fail the request
		// You might want to add proper logging here
		fmt.Printf("Failed to create follow notification: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully followed user"})
}

func (uc *UserController) UnfollowUser(c *gin.Context) {
	followerID := c.GetString("user_id")
	followingID := c.Param("user_id")

	var follow models.Follow
	if err := uc.db.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&follow).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Follow relationship not found"})
		return
	}

	if err := uc.db.Delete(&follow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unfollow user"})
		return
	}

	// Update follower and following counts
	uc.db.Model(&models.User{}).Where("id = ?", followerID).UpdateColumn("following_count", gorm.Expr("following_count - ?", 1))
	uc.db.Model(&models.User{}).Where("id = ?", followingID).UpdateColumn("followers_count", gorm.Expr("followers_count - ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Successfully unfollowed user"})
}

// Check following status
func (uc *UserController) GetFollowingStatus(c *gin.Context) {
	followerID := c.GetString("user_id")
	followingID := c.Param("user_id")

	if followerID == followingID {
		c.JSON(http.StatusOK, gin.H{
			"is_following": false,
			"message":      "Cannot follow yourself",
		})
		return
	}

	// Check if target user exists
	var targetUser models.User
	if err := uc.db.First(&targetUser, "id = ?", followingID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check follow status
	var follow models.Follow
	isFollowing := false
	if err := uc.db.Where("follower_id = ? AND following_id = ?", followerID, followingID).First(&follow).Error; err == nil {
		isFollowing = true
	}

	c.JSON(http.StatusOK, gin.H{
		"is_following": isFollowing,
		"user": gin.H{
			"id":     targetUser.ID,
			"name":   targetUser.Name,
			"handle": targetUser.Handle,
			"avatar": targetUser.Avatar,
		},
	})
}

func (uc *UserController) GetFollowers(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var follows []models.Follow
	if err := uc.db.Preload("Follower").Where("following_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&follows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch followers"})
		return
	}

	followers := make([]models.User, 0, len(follows))
	for _, follow := range follows {
		follow.Follower.Password = ""
		followers = append(followers, follow.Follower)
	}

	c.JSON(http.StatusOK, followers)
}

func (uc *UserController) GetFollowing(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var follows []models.Follow
	if err := uc.db.Preload("Following").Where("follower_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&follows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch following"})
		return
	}

	following := make([]models.User, 0, len(follows))
	for _, follow := range follows {
		follow.Following.Password = ""
		following = append(following, follow.Following)
	}

	c.JSON(http.StatusOK, following)
}

// Search users by name or handle
func (uc *UserController) SearchUsers(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var users []models.User
	searchPattern := "%" + query + "%"

	if err := uc.db.Where("name LIKE ? OR handle LIKE ?", searchPattern, searchPattern).
		Order("followers_count DESC").Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search users"})
		return
	}

	// Remove passwords
	for i := range users {
		users[i].Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"page":  page,
		"limit": limit,
	})
}

// Get user by handle
func (uc *UserController) GetUserByHandle(c *gin.Context) {
	handle := c.Param("handle")
	currentUserID := c.GetString("user_id")

	var user models.User
	if err := uc.db.Preload("Motorcycles").Where("handle = ?", handle).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.Password = ""

	// Check if current user is following this user
	isFollowing := false
	if currentUserID != user.ID {
		var follow models.Follow
		if err := uc.db.Where("follower_id = ? AND following_id = ?", currentUserID, user.ID).First(&follow).Error; err == nil {
			isFollowing = true
		}
	}

	response := gin.H{
		"user":         user,
		"is_following": isFollowing,
	}

	c.JSON(http.StatusOK, response)
}

// Helper function to generate unique handle
func (uc *UserController) GenerateUniqueHandle(baseName string) string {
	baseHandle := models.GenerateHandleFromName(baseName)
	handle := baseHandle
	counter := 1

	for {
		var existingUser models.User
		if err := uc.db.Where("handle = ?", handle).First(&existingUser).Error; err != nil {
			// Handle is available
			break
		}
		// Handle exists, try with number suffix
		handle = fmt.Sprintf("%s_%d", baseHandle, counter)
		counter++
	}

	return handle
}
