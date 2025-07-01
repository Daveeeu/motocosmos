// File: /controllers/route_controller.go
package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"strconv"
)

type RouteController struct {
	db *gorm.DB
}

func NewRouteController(db *gorm.DB) *RouteController {
	return &RouteController{db: db}
}

type CreateRouteRequest struct {
	Name          string                 `json:"name" binding:"required"`
	Description   string                 `json:"description"`
	Difficulty    string                 `json:"difficulty"`
	Tags          []string               `json:"tags"`
	Waypoints     []RouteWaypointRequest `json:"waypoints" binding:"required"`
	RouteGeometry []map[string]float64   `json:"route_geometry"`
	RouteSettings map[string]interface{} `json:"route_settings"`
	IsPublic      bool                   `json:"is_public"`
}

type RouteWaypointRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Latitude    float64 `json:"latitude" binding:"required"`
	Longitude   float64 `json:"longitude" binding:"required"`
	Order       int     `json:"order" binding:"required"`
}

func (rc *RouteController) GetRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var routes []models.Route
	query := rc.db.Preload("Waypoints").Where("user_id = ? OR is_public = ?", userID, true)

	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}

	if err := query.Offset(offset).Limit(limit).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch routes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"routes": routes,
		"page":   page,
		"limit":  limit,
	})
}

func (rc *RouteController) CreateRoute(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate total distance from waypoints
	totalDistance := rc.calculateTotalDistance(req.Waypoints)

	route := models.Route{
		ID:            uuid.New().String(),
		UserID:        userID,
		Name:          req.Name,
		Description:   req.Description,
		TotalDistance: totalDistance,
		EstimatedTime: int(totalDistance * 60), // Rough estimate: 1 minute per km
		Difficulty:    req.Difficulty,
		Tags:          models.StringSlice(req.Tags),
		IsPublic:      req.IsPublic,
		RouteGeometry: convertToJSONData(req.RouteGeometry),
		RouteSettings: models.JSONData(req.RouteSettings),
	}

	if err := rc.db.Create(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create route"})
		return
	}

	// Create waypoints
	for _, wp := range req.Waypoints {
		waypoint := models.RouteWaypoint{
			RouteID:     route.ID,
			Name:        wp.Name,
			Description: wp.Description,
			Latitude:    wp.Latitude,
			Longitude:   wp.Longitude,
			Order:       wp.Order,
		}
		rc.db.Create(&waypoint)
	}

	// Load the complete route with waypoints
	rc.db.Preload("Waypoints").First(&route, "id = ?", route.ID)

	c.JSON(http.StatusCreated, route)
}

func convertToJSONData(geometry []map[string]float64) models.JSONData {
	result := make(models.JSONData)
	for i, point := range geometry {
		result[fmt.Sprintf("%d", i)] = point
	}
	return result
}

func (rc *RouteController) GetRoute(c *gin.Context) {
	routeID := c.Param("id")

	var route models.Route
	if err := rc.db.Preload("Waypoints").Preload("User").First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Increment times used if user is loading their own route
	userID := c.GetString("user_id")
	if route.UserID == userID {
		rc.db.Model(&route).UpdateColumn("times_used", gorm.Expr("times_used + ?", 1))
	}

	route.User.Password = ""
	c.JSON(http.StatusOK, route)
}

func (rc *RouteController) UpdateRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.Route
	if err := rc.db.First(&route, "id = ? AND user_id = ?", routeID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found or access denied"})
		return
	}

	var req CreateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update route
	totalDistance := rc.calculateTotalDistance(req.Waypoints)
	updates := map[string]interface{}{
		"name":           req.Name,
		"description":    req.Description,
		"total_distance": totalDistance,
		"estimated_time": int(totalDistance * 60),
		"difficulty":     req.Difficulty,
		"tags":           models.StringSlice(req.Tags),
		"is_public":      req.IsPublic,
		"route_geometry": convertToJSONData(req.RouteGeometry),
		"route_settings": models.JSONData(req.RouteSettings),
	}

	if err := rc.db.Model(&route).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update route"})
		return
	}

	// Delete old waypoints and create new ones
	rc.db.Where("route_id = ?", routeID).Delete(&models.RouteWaypoint{})
	for _, wp := range req.Waypoints {
		waypoint := models.RouteWaypoint{
			RouteID:     routeID,
			Name:        wp.Name,
			Description: wp.Description,
			Latitude:    wp.Latitude,
			Longitude:   wp.Longitude,
			Order:       wp.Order,
		}
		rc.db.Create(&waypoint)
	}

	// Return updated route
	rc.db.Preload("Waypoints").First(&route, "id = ?", routeID)
	c.JSON(http.StatusOK, route)
}

func (rc *RouteController) DeleteRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.Route
	if err := rc.db.First(&route, "id = ? AND user_id = ?", routeID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found or access denied"})
		return
	}

	// Delete waypoints first
	rc.db.Where("route_id = ?", routeID).Delete(&models.RouteWaypoint{})

	// Delete saved routes
	rc.db.Where("route_id = ?", routeID).Delete(&models.SavedRoute{})

	// Delete the route
	if err := rc.db.Delete(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route deleted successfully"})
}

func (rc *RouteController) SaveRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	// Check if route exists
	var route models.Route
	if err := rc.db.First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Check if already saved
	var existingSave models.SavedRoute
	if err := rc.db.Where("user_id = ? AND route_id = ?", userID, routeID).First(&existingSave).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Route already saved"})
		return
	}

	// Save the route
	savedRoute := models.SavedRoute{
		UserID:  userID,
		RouteID: routeID,
	}

	if err := rc.db.Create(&savedRoute).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route saved successfully"})
}

func (rc *RouteController) GetSavedRoutes(c *gin.Context) {
	userID := c.GetString("user_id")

	var savedRoutes []models.SavedRoute
	if err := rc.db.Preload("Route").Preload("Route.Waypoints").Where("user_id = ?", userID).Find(&savedRoutes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch saved routes"})
		return
	}

	var routes []models.Route
	for _, saved := range savedRoutes {
		routes = append(routes, saved.Route)
	}

	c.JSON(http.StatusOK, routes)
}

func (rc *RouteController) GetRecommendations(c *gin.Context) {
	userID := c.GetString("user_id")

	// Get popular public routes
	var routes []models.Route
	if err := rc.db.Preload("Waypoints").Where("is_public = ? AND user_id != ?", true, userID).
		Order("times_used DESC").Limit(10).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recommendations"})
		return
	}

	c.JSON(http.StatusOK, routes)
}

func (rc *RouteController) PlanRoute(c *gin.Context) {
	var req struct {
		Waypoints     []RouteWaypointRequest `json:"waypoints" binding:"required"`
		AvoidHighways bool                   `json:"avoid_highways"`
		PreferWinding bool                   `json:"prefer_winding"`
		Profile       string                 `json:"profile"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real implementation, this would call Mapbox API
	// For now, return a mock response
	totalDistance := rc.calculateTotalDistance(req.Waypoints)

	response := gin.H{
		"geometry": []map[string]float64{}, // Would contain route geometry
		"distance": totalDistance * 1000,   // in meters
		"duration": totalDistance * 60,     // rough estimate in seconds
		"summary":  "Planned route",
		"steps":    []gin.H{}, // Would contain turn-by-turn instructions
	}

	c.JSON(http.StatusOK, response)
}

func (rc *RouteController) CalculateMetrics(c *gin.Context) {
	var req struct {
		Start LatLng `json:"start" binding:"required"`
		End   LatLng `json:"end" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	distance := rc.calculateDistance(req.Start.Latitude, req.Start.Longitude, req.End.Latitude, req.End.Longitude)
	duration := distance * 60 // rough estimate: 1 minute per km

	c.JSON(http.StatusOK, gin.H{
		"distance": distance * 1000, // in meters
		"duration": duration,        // in seconds
	})
}

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func (rc *RouteController) calculateTotalDistance(waypoints []RouteWaypointRequest) float64 {
	if len(waypoints) < 2 {
		return 0
	}

	var totalDistance float64
	for i := 0; i < len(waypoints)-1; i++ {
		distance := rc.calculateDistance(
			waypoints[i].Latitude, waypoints[i].Longitude,
			waypoints[i+1].Latitude, waypoints[i+1].Longitude,
		)
		totalDistance += distance
	}

	return totalDistance
}

func (rc *RouteController) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Haversine formula implementation
	const earthRadius = 6371 // km

	dLat := (lat2 - lat1) * (3.14159265359 / 180)
	dLon := (lon2 - lon1) * (3.14159265359 / 180)

	a := 0.5 - 0.5*(dLat*dLat+(1-lat1*lat1)*(1-lat2*lat2)*dLon*dLon)

	return earthRadius * 2 * (a * (1 - a))
}
