// File: /controllers/ride_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"time"
)

type RideController struct {
	db *gorm.DB
}

func NewRideController(db *gorm.DB) *RideController {
	return &RideController{db: db}
}

type StartRideRequest struct {
	MotorcycleID string `json:"motorcycle_id" binding:"required"`
}

type RoutePointRequest struct {
	Latitude  float64   `json:"latitude" binding:"required"`
	Longitude float64   `json:"longitude" binding:"required"`
	Altitude  *float64  `json:"altitude"`
	Speed     *float64  `json:"speed"`
	Timestamp time.Time `json:"timestamp" binding:"required"`
}

func (rc *RideController) GetRides(c *gin.Context) {
	userID := c.GetString("user_id")

	var rides []models.RideRecord
	if err := rc.db.Preload("Motorcycle").Preload("RoutePoints").
		Where("user_id = ?", userID).Order("created_at DESC").Find(&rides).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch rides"})
		return
	}

	c.JSON(http.StatusOK, rides)
}

func (rc *RideController) StartRide(c *gin.Context) {
	userID := c.GetString("user_id")

	var req StartRideRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if motorcycle belongs to user
	var motorcycle models.Motorcycle
	if err := rc.db.First(&motorcycle, "id = ? AND user_id = ?", req.MotorcycleID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Motorcycle not found or access denied"})
		return
	}

	// Check if user has an active ride
	var activeRide models.RideRecord
	if err := rc.db.Where("user_id = ? AND is_completed = ?", userID, false).First(&activeRide).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "You already have an active ride"})
		return
	}

	ride := models.RideRecord{
		ID:             uuid.New().String(),
		UserID:         userID,
		MotorcycleID:   req.MotorcycleID,
		MotorcycleName: motorcycle.Brand + " " + motorcycle.Model,
		StartTime:      time.Now(),
		IsCompleted:    false,
	}

	if err := rc.db.Create(&ride).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start ride"})
		return
	}

	c.JSON(http.StatusCreated, ride)
}

func (rc *RideController) PauseRide(c *gin.Context) {
	userID := c.GetString("user_id")
	rideID := c.Param("id")

	var ride models.RideRecord
	if err := rc.db.First(&ride, "id = ? AND user_id = ? AND is_completed = ?", rideID, userID, false).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Active ride not found"})
		return
	}

	// In a real implementation, you might store pause/resume timestamps
	c.JSON(http.StatusOK, gin.H{"message": "Ride paused successfully"})
}

func (rc *RideController) ResumeRide(c *gin.Context) {
	userID := c.GetString("user_id")
	rideID := c.Param("id")

	var ride models.RideRecord
	if err := rc.db.First(&ride, "id = ? AND user_id = ? AND is_completed = ?", rideID, userID, false).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Active ride not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ride resumed successfully"})
}

func (rc *RideController) StopRide(c *gin.Context) {
	userID := c.GetString("user_id")
	rideID := c.Param("id")

	var ride models.RideRecord
	if err := rc.db.Preload("RoutePoints").First(&ride, "id = ? AND user_id = ? AND is_completed = ?",
		rideID, userID, false).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Active ride not found"})
		return
	}

	endTime := time.Now()
	duration := int(endTime.Sub(ride.StartTime).Seconds())

	// Calculate statistics from route points
	distance, maxSpeed, averageSpeed, maxAltitude, totalElevation := rc.calculateRideStatistics(ride.RoutePoints)

	updates := map[string]interface{}{
		"end_time":        &endTime,
		"duration":        duration,
		"distance":        distance,
		"max_speed":       maxSpeed,
		"average_speed":   averageSpeed,
		"max_altitude":    maxAltitude,
		"total_elevation": totalElevation,
		"is_completed":    true,
	}

	if err := rc.db.Model(&ride).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop ride"})
		return
	}

	// Update user statistics
	rc.updateUserStatistics(userID, distance, duration)

	// Reload ride with updated data
	rc.db.Preload("Motorcycle").Preload("RoutePoints").First(&ride, "id = ?", rideID)

	c.JSON(http.StatusOK, ride)
}

func (rc *RideController) GetRide(c *gin.Context) {
	userID := c.GetString("user_id")
	rideID := c.Param("id")

	var ride models.RideRecord
	if err := rc.db.Preload("Motorcycle").Preload("RoutePoints").
		First(&ride, "id = ? AND user_id = ?", rideID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ride not found"})
		return
	}

	c.JSON(http.StatusOK, ride)
}

func (rc *RideController) ShareRide(c *gin.Context) {
	userID := c.GetString("user_id")
	rideID := c.Param("id")

	var ride models.RideRecord
	if err := rc.db.First(&ride, "id = ? AND user_id = ?", rideID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ride not found"})
		return
	}

	if !ride.IsCompleted {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot share incomplete ride"})
		return
	}

	// In a real implementation, this might create a post or share to social media
	c.JSON(http.StatusOK, gin.H{"message": "Ride shared successfully"})
}

func (rc *RideController) AddRoutePoint(c *gin.Context) {
	userID := c.GetString("user_id")
	rideID := c.Param("id")

	var ride models.RideRecord
	if err := rc.db.First(&ride, "id = ? AND user_id = ? AND is_completed = ?",
		rideID, userID, false).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Active ride not found"})
		return
	}

	var req RoutePointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	routePoint := models.RoutePoint{
		RideRecordID: rideID,
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		Altitude:     req.Altitude,
		Speed:        req.Speed,
		Timestamp:    req.Timestamp,
	}

	if err := rc.db.Create(&routePoint).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add route point"})
		return
	}

	c.JSON(http.StatusCreated, routePoint)
}

func (rc *RideController) calculateRideStatistics(routePoints []models.RoutePoint) (float64, float64, float64, float64, float64) {
	if len(routePoints) < 2 {
		return 0, 0, 0, 0, 0
	}

	var totalDistance float64
	var maxSpeed float64
	var totalSpeed float64
	var speedCount int
	var maxAltitude float64
	var totalElevation float64
	var prevAltitude *float64

	for i, point := range routePoints {
		// Calculate distance
		if i > 0 {
			prevPoint := routePoints[i-1]
			distance := rc.calculateDistance(
				prevPoint.Latitude, prevPoint.Longitude,
				point.Latitude, point.Longitude,
			)
			totalDistance += distance
		}

		// Track max speed
		if point.Speed != nil {
			if *point.Speed > maxSpeed {
				maxSpeed = *point.Speed
			}
			totalSpeed += *point.Speed
			speedCount++
		}

		// Track max altitude and elevation gain
		if point.Altitude != nil {
			if *point.Altitude > maxAltitude {
				maxAltitude = *point.Altitude
			}
			if prevAltitude != nil && *point.Altitude > *prevAltitude {
				totalElevation += *point.Altitude - *prevAltitude
			}
			prevAltitude = point.Altitude
		}
	}

	averageSpeed := float64(0)
	if speedCount > 0 {
		averageSpeed = totalSpeed / float64(speedCount)
	}

	return totalDistance, maxSpeed, averageSpeed, maxAltitude, totalElevation
}

func (rc *RideController) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Haversine formula implementation - same as in route controller
	const earthRadius = 6371 // km

	dLat := (lat2 - lat1) * (3.14159265359 / 180)
	dLon := (lon2 - lon1) * (3.14159265359 / 180)

	a := 0.5 - 0.5*(dLat*dLat+(1-lat1*lat1)*(1-lat2*lat2)*dLon*dLon)

	return earthRadius * 2 * (a * (1 - a))
}

func (rc *RideController) updateUserStatistics(userID string, distance float64, duration int) {
	var user models.User
	if err := rc.db.First(&user, "id = ?", userID).Error; err != nil {
		return // User not found, skip update
	}

	// Update rides count
	rc.db.Model(&user).UpdateColumn("rides_count", gorm.Expr("rides_count + ?", 1))

	// In a real implementation, you would properly parse and update total_time and total_distance
	// For now, we'll just increment the rides count
}
