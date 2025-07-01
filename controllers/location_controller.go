// File: /controllers/location_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"math"
	"motocosmos-api/models"
	"net/http"
	"strconv"
)

type LocationController struct {
	db *gorm.DB
}

func NewLocationController(db *gorm.DB) *LocationController {
	return &LocationController{db: db}
}

type UpdateLocationRequest struct {
	Latitude         float64 `json:"latitude" binding:"required"`
	Longitude        float64 `json:"longitude" binding:"required"`
	IsLocationPublic bool    `json:"is_location_public"`
	Status           string  `json:"status"`
}

func (lc *LocationController) UpdateLocation(c *gin.Context) {
	userID := c.GetString("user_id")

	var req UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info
	var user models.User
	if err := lc.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update or create user location
	var userLocation models.UserLocation
	if err := lc.db.Where("user_id = ?", userID).First(&userLocation).Error; err != nil {
		// Create new location record
		userLocation = models.UserLocation{
			ID:               userID + "_location",
			UserID:           userID,
			Username:         user.Name,
			Latitude:         req.Latitude,
			Longitude:        req.Longitude,
			IsOnline:         true,
			Status:           req.Status,
			IsLocationPublic: req.IsLocationPublic,
		}
		lc.db.Create(&userLocation)
	} else {
		// Update existing location
		updates := map[string]interface{}{
			"latitude":           req.Latitude,
			"longitude":          req.Longitude,
			"is_online":          true,
			"last_seen":          lc.db.NowFunc(),
			"status":             req.Status,
			"is_location_public": req.IsLocationPublic,
		}
		lc.db.Model(&userLocation).Updates(updates)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Location updated successfully"})
}

func (lc *LocationController) GetNearbyUsers(c *gin.Context) {
	userID := c.GetString("user_id")
	radiusStr := c.DefaultQuery("radius", "10") // Default 10km radius
	radius, _ := strconv.ParseFloat(radiusStr, 64)

	// Get current user location
	var currentUserLocation models.UserLocation
	if err := lc.db.Where("user_id = ?", userID).First(&currentUserLocation).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User location not found. Please update your location first."})
		return
	}

	// Get all public locations
	var userLocations []models.UserLocation
	if err := lc.db.Preload("User").Where("user_id != ? AND is_location_public = ? AND is_online = ?",
		userID, true, true).Find(&userLocations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nearby users"})
		return
	}

	// Filter by distance and calculate distance
	var nearbyUsers []gin.H
	for _, location := range userLocations {
		distance := lc.calculateDistance(
			currentUserLocation.Latitude, currentUserLocation.Longitude,
			location.Latitude, location.Longitude,
		)

		if distance <= radius {
			location.User.Password = "" // Remove password from response
			nearbyUsers = append(nearbyUsers, gin.H{
				"user":        location.User,
				"latitude":    location.Latitude,
				"longitude":   location.Longitude,
				"status":      location.Status,
				"distance_km": distance,
				"last_seen":   location.UpdatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"nearby_users": nearbyUsers,
		"radius_km":    radius,
		"count":        len(nearbyUsers),
	})
}

func (lc *LocationController) GetFriends(c *gin.Context) {
	userID := c.GetString("user_id")

	// Get user's friends who have shared their location
	var follows []models.Follow
	if err := lc.db.Preload("Following").Where("follower_id = ?", userID).Find(&follows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch friends"})
		return
	}

	var friendsWithLocation []gin.H
	for _, follow := range follows {
		var friendLocation models.UserLocation
		if err := lc.db.Where("user_id = ? AND is_location_public = ?",
			follow.FollowingID, true).First(&friendLocation).Error; err == nil {

			follow.Following.Password = ""
			friendsWithLocation = append(friendsWithLocation, gin.H{
				"user":      follow.Following,
				"latitude":  friendLocation.Latitude,
				"longitude": friendLocation.Longitude,
				"status":    friendLocation.Status,
				"is_online": friendLocation.IsOnline,
				"last_seen": friendLocation.UpdatedAt,
			})
		}
	}

	c.JSON(http.StatusOK, friendsWithLocation)
}

func (lc *LocationController) AddFriend(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("id")

	if userID == targetUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot add yourself as friend"})
		return
	}

	// Check if target user exists
	var targetUser models.User
	if err := lc.db.First(&targetUser, "id = ?", targetUserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if already following
	var existingFollow models.Follow
	if err := lc.db.Where("follower_id = ? AND following_id = ?", userID, targetUserID).First(&existingFollow).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already following this user"})
		return
	}

	// Create follow relationship
	follow := models.Follow{
		FollowerID:  userID,
		FollowingID: targetUserID,
	}

	if err := lc.db.Create(&follow).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add friend"})
		return
	}

	// Update counters
	lc.db.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("following_count", gorm.Expr("following_count + ?", 1))
	lc.db.Model(&models.User{}).Where("id = ?", targetUserID).UpdateColumn("followers_count", gorm.Expr("followers_count + ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Friend added successfully"})
}

func (lc *LocationController) RemoveFriend(c *gin.Context) {
	userID := c.GetString("user_id")
	targetUserID := c.Param("id")

	if err := lc.db.Where("follower_id = ? AND following_id = ?", userID, targetUserID).Delete(&models.Follow{}).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Friend relationship not found"})
		return
	}

	// Update counters
	lc.db.Model(&models.User{}).Where("id = ?", userID).UpdateColumn("following_count", gorm.Expr("following_count - ?", 1))
	lc.db.Model(&models.User{}).Where("id = ?", targetUserID).UpdateColumn("followers_count", gorm.Expr("followers_count - ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Friend removed successfully"})
}

func (lc *LocationController) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371 // km

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return distance
}
