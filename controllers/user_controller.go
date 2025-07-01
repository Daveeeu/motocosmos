// File: /controllers/user_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
)

type UserController struct {
	db *gorm.DB
}

func NewUserController(db *gorm.DB) *UserController {
	return &UserController{db: db}
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
		Name   *string `json:"name"`
		Avatar *string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Avatar != nil {
		updates["avatar"] = *req.Avatar
	}

	if err := uc.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
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

	// Get ride statistics
	var ridesCount int64
	uc.db.Model(&models.RideRecord{}).Where("user_id = ? AND is_completed = ?", userID, true).Count(&ridesCount)

	statistics := gin.H{
		"followers_count": user.FollowersCount,
		"following_count": user.FollowingCount,
		"rides_count":     ridesCount,
		"total_time":      user.TotalTime,
		"total_distance":  user.TotalDistance,
	}

	c.JSON(http.StatusOK, statistics)
}

func (uc *UserController) FollowUser(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("id")

	if userID == targetUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot follow yourself"})
		return
	}

	// Check if already following
	var existingFollow models.Follow
	if err := uc.db.Where("follower_id = ? AND following_id = ?", userID, targetUserID).First(&existingFollow).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already following this user"})
		return
	}

	// Create follow relationship
	follow := models.Follow{
		FollowerID:  userID,
		FollowingID: targetUserID,
	}

	if err := uc.db.Create(&follow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to follow user"})
		return
	}

	// Update counters
	uc.db.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("following_count", gorm.Expr("following_count + ?", 1))
	uc.db.Model(&models.User{}).Where("id = ?", targetUserID).UpdateColumn("followers_count", gorm.Expr("followers_count + ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Successfully followed user"})
}

func (uc *UserController) UnfollowUser(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("id")

	if err := uc.db.Where("follower_id = ? AND following_id = ?", userID, targetUserID).Delete(&models.Follow{}).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Follow relationship not found"})
		return
	}

	// Update counters
	uc.db.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("following_count", gorm.Expr("following_count - ?", 1))
	uc.db.Model(&models.User{}).Where("id = ?", targetUserID).UpdateColumn("followers_count", gorm.Expr("followers_count - ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Successfully unfollowed user"})
}

func (uc *UserController) GetFollowers(c *gin.Context) {
	userID := c.GetString("user_id")

	var follows []models.Follow
	if err := uc.db.Preload("Follower").Where("following_id = ?", userID).Find(&follows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get followers"})
		return
	}

	var followers []models.User
	for _, follow := range follows {
		follow.Follower.Password = ""
		followers = append(followers, follow.Follower)
	}

	c.JSON(http.StatusOK, followers)
}

func (uc *UserController) GetFollowing(c *gin.Context) {
	userID := c.GetString("user_id")

	var follows []models.Follow
	if err := uc.db.Preload("Following").Where("follower_id = ?", userID).Find(&follows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get following"})
		return
	}

	var following []models.User
	for _, follow := range follows {
		follow.Following.Password = ""
		following = append(following, follow.Following)
	}

	c.JSON(http.StatusOK, following)
}
