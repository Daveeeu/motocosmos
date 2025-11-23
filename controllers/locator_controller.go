// File: /controllers/locator_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"motocosmos-api/repositories"
	"motocosmos-api/services"
	"net/http"
)

type LocatorController struct {
	db              *gorm.DB
	locationService *services.LocationService
}

func NewLocatorController(db *gorm.DB) *LocatorController {
	locationRepo := repositories.NewLocationRepository(db)
	locationService := services.NewLocationService(locationRepo)

	return &LocatorController{
		db:              db,
		locationService: locationService,
	}
}

// GetLocator handles GET /api/v1/locator
// Returns all visible friends on the map
func (lc *LocatorController) GetLocator(c *gin.Context) {
	userID := c.GetString("user_id")

	locatorData, err := lc.locationService.GetLocatorData(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve locator data",
		})
		return
	}

	c.JSON(http.StatusOK, locatorData)
}

// UpdateLocation handles POST /api/v1/locator/location
// Updates current user's location and availability
func (lc *LocatorController) UpdateLocation(c *gin.Context) {
	userID := c.GetString("user_id")

	var req models.UpdateLocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Get user info for username
	var user models.User
	if err := lc.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update location
	if err := lc.locationService.UpdateLocation(userID, req, &user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update location",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Location updated successfully",
	})
}

// UpdateVisibility handles POST /api/v1/locator/visibility
// Updates location visibility settings
func (lc *LocatorController) UpdateVisibility(c *gin.Context) {
	userID := c.GetString("user_id")

	var req models.UpdateVisibilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request",
			"message": err.Error(),
		})
		return
	}

	// Validate that if mode is custom, allowed_user_ids is provided
	if req.VisibilityMode == "custom" && len(req.AllowedUserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Custom visibility mode requires at least one allowed user",
		})
		return
	}

	if err := lc.locationService.UpdateVisibilitySettings(userID, req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update visibility settings",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Visibility settings updated successfully",
	})
}

// GetVisibilitySettings handles GET /api/v1/locator/settings
// Returns current visibility settings
func (lc *LocatorController) GetVisibilitySettings(c *gin.Context) {
	userID := c.GetString("user_id")

	settings, err := lc.locationService.GetVisibilitySettings(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to retrieve visibility settings",
		})
		return
	}

	c.JSON(http.StatusOK, settings)
}

// GetNearbyUsers handles GET /api/v1/locator/nearby
// Returns nearby users within a specified radius (legacy endpoint, kept for compatibility)
func (lc *LocatorController) GetNearbyUsers(c *gin.Context) {
	// This can redirect to GetLocator or implement radius-based filtering
	// For now, redirect to main locator endpoint
	lc.GetLocator(c)
}