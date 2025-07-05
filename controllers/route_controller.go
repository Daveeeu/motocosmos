// File: /controllers/route_controller.go
package controllers

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"math"
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
	Name               string                  `json:"name" binding:"required"`
	Description        string                  `json:"description"`
	Difficulty         string                  `json:"difficulty"`
	Tags               []string                `json:"tags"`
	Waypoints          []RouteWaypointRequestV `json:"waypoints" binding:"required"`
	RouteGeometry      []map[string]float64    `json:"route_geometry"` // ⭐ JAVÍTOTT: Megfelelő típus
	RouteSettings      map[string]interface{}  `json:"route_settings"`
	IsPublic           bool                    `json:"is_public"`
	TotalDistance      float64                 `json:"total_distance"`
	TotalElevation     float64                 `json:"total_elevation"`
	EstimatedTime      int                     `json:"estimated_time"` // in seconds
	AvoidHighways      bool                    `json:"avoid_highways"`
	PreferWindingRoads bool                    `json:"prefer_winding_roads"`
}

type RouteWaypointRequestV struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Latitude    float64 `json:"latitude" binding:"required"`
	Longitude   float64 `json:"longitude" binding:"required"`
	Order       int     `json:"order" binding:"required"`
}

// GetRoutes returns user's personal routes with pagination and filtering
func (rc *RouteController) GetRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var routes []models.Route
	var total int64

	query := rc.db.Preload("Waypoints").Where("user_id = ?", userID)

	if search := c.Query("search"); search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}

	// Get total count
	query.Model(&models.Route{}).Count(&total)

	// Get paginated results
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch routes"})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	c.JSON(http.StatusOK, gin.H{
		"routes":      routes,
		"page":        page,
		"limit":       limit,
		"total":       total,
		"has_more":    hasMore,
		"total_pages": totalPages,
	})
}

// CreateRoute creates a new personal route (saved from route planning)
func (rc *RouteController) CreateRoute(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate total distance if not provided
	totalDistance := req.TotalDistance
	if totalDistance == 0 {
		totalDistance = rc.calculateTotalDistance(req.Waypoints)
	}

	// Calculate estimated time if not provided
	estimatedTime := req.EstimatedTime
	if estimatedTime == 0 {
		estimatedTime = int(totalDistance * 60) // Rough estimate: 1 minute per km
	}

	// Create route settings
	routeSettings := map[string]interface{}{
		"avoid_highways":       req.AvoidHighways,
		"prefer_winding_roads": req.PreferWindingRoads,
		"profile":              "driving",
		"saved_at":             fmt.Sprintf("%d", rc.getCurrentTimestamp()),
	}
	if req.RouteSettings != nil {
		for k, v := range req.RouteSettings {
			routeSettings[k] = v
		}
	}

	route := models.Route{
		ID:             uuid.New().String(),
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		TotalDistance:  totalDistance,
		TotalElevation: req.TotalElevation,
		EstimatedTime:  estimatedTime,
		Difficulty:     req.Difficulty,
		Tags:           models.StringSlice(req.Tags),
		IsPublic:       req.IsPublic,
		RouteGeometry:  rc.convertGeometryToJSONData(req.RouteGeometry), // ⭐ JAVÍTOTT: Új metódus
		RouteSettings:  models.JSONData(routeSettings),
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
		if err := rc.db.Create(&waypoint).Error; err != nil {
			// Log error but continue
			fmt.Printf("Warning: Could not create waypoint: %v\n", err)
		}
	}

	// Load the complete route with waypoints
	rc.db.Preload("Waypoints").First(&route, "id = ?", route.ID)

	c.JSON(http.StatusCreated, route)
}

// GetRoute returns a single route by ID
func (rc *RouteController) GetRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.Route
	if err := rc.db.Preload("Waypoints").Preload("User").First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Check if user owns this route or if it's public
	if route.UserID != userID && !route.IsPublic {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Increment times used if user is loading their own route
	if route.UserID == userID {
		rc.db.Model(&route).UpdateColumn("times_used", gorm.Expr("times_used + ?", 1))
	}

	route.User.Password = ""
	c.JSON(http.StatusOK, route)
}

// UpdateRoute updates an existing route (owner only)
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

	// Calculate total distance if not provided
	totalDistance := req.TotalDistance
	if totalDistance == 0 {
		totalDistance = rc.calculateTotalDistance(req.Waypoints)
	}

	// Calculate estimated time if not provided
	estimatedTime := req.EstimatedTime
	if estimatedTime == 0 {
		estimatedTime = int(totalDistance * 60)
	}

	// Update route settings
	routeSettings := map[string]interface{}{
		"avoid_highways":       req.AvoidHighways,
		"prefer_winding_roads": req.PreferWindingRoads,
		"profile":              "driving",
		"updated_at":           fmt.Sprintf("%d", rc.getCurrentTimestamp()),
	}
	if req.RouteSettings != nil {
		for k, v := range req.RouteSettings {
			routeSettings[k] = v
		}
	}

	// Update route
	updates := map[string]interface{}{
		"name":            req.Name,
		"description":     req.Description,
		"total_distance":  totalDistance,
		"total_elevation": req.TotalElevation,
		"estimated_time":  estimatedTime,
		"difficulty":      req.Difficulty,
		"tags":            models.StringSlice(req.Tags),
		"is_public":       req.IsPublic,
		"route_geometry":  rc.convertGeometryToJSONData(req.RouteGeometry), // ⭐ JAVÍTOTT: Új metódus
		"route_settings":  models.JSONData(routeSettings),
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

// ⭐ ÚJ METÓDUS: Route geometry konvertálása JSONData formátumba
func (rc *RouteController) convertGeometryToJSONData(geometry []map[string]float64) models.JSONData {
	result := make(models.JSONData)
	for i, point := range geometry {
		result[fmt.Sprintf("%d", i)] = point
	}
	return result
}

// GetSavedRoutes returns routes that the user has saved (their own routes)
func (rc *RouteController) GetSavedRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var routes []models.Route
	var total int64

	// Get user's own routes (saved routes are the user's personal routes)
	query := rc.db.Preload("Waypoints").Where("user_id = ?", userID)

	// Get total count
	query.Model(&models.Route{}).Count(&total)

	// Get paginated results
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch saved routes"})
		return
	}

	c.JSON(http.StatusOK, routes)
}

// BookmarkRoute allows users to bookmark other users' public routes
func (rc *RouteController) BookmarkRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	// Check if route exists and is public
	var route models.Route
	if err := rc.db.First(&route, "id = ? AND (is_public = ? OR user_id = ?)", routeID, true, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found or not accessible"})
		return
	}

	// Check if already bookmarked
	var existingBookmark models.SavedRoute
	if err := rc.db.Where("user_id = ? AND route_id = ?", userID, routeID).First(&existingBookmark).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Route already bookmarked"})
		return
	}

	// Create bookmark
	bookmark := models.SavedRoute{
		UserID:  userID,
		RouteID: routeID,
	}

	if err := rc.db.Create(&bookmark).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bookmark route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Route bookmarked successfully"})
}

// UnbookmarkRoute removes a route bookmark
func (rc *RouteController) UnbookmarkRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var bookmark models.SavedRoute
	if err := rc.db.Where("user_id = ? AND route_id = ?", userID, routeID).First(&bookmark).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Bookmark not found"})
		return
	}

	if err := rc.db.Delete(&bookmark).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove bookmark"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Bookmark removed successfully"})
}

// GetBookmarkedRoutes returns routes that the user has bookmarked
func (rc *RouteController) GetBookmarkedRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var bookmarks []models.SavedRoute
	if err := rc.db.Preload("Route").Preload("Route.Waypoints").Where("user_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&bookmarks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookmarked routes"})
		return
	}

	var routes []models.Route
	for _, bookmark := range bookmarks {
		routes = append(routes, bookmark.Route)
	}

	c.JSON(http.StatusOK, routes)
}

// GetRecommendations returns recommended routes (popular public routes)
func (rc *RouteController) GetRecommendations(c *gin.Context) {
	userID := c.GetString("user_id")

	var routes []models.Route
	if err := rc.db.Preload("Waypoints").Where("is_public = ? AND user_id != ?", true, userID).
		Order("times_used DESC").Limit(10).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch recommendations"})
		return
	}

	c.JSON(http.StatusOK, routes)
}

// PlanRoute - Mock implementation for route planning (would use Mapbox API in production)
func (rc *RouteController) PlanRoute(c *gin.Context) {
	var req struct {
		Waypoints     []RouteWaypointRequestV `json:"waypoints" binding:"required"`
		AvoidHighways bool                    `json:"avoid_highways"`
		PreferWinding bool                    `json:"prefer_winding"`
		Profile       string                  `json:"profile"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real implementation, this would call Mapbox API
	totalDistance := rc.calculateTotalDistance(req.Waypoints)

	// Mock geometry - in production this would come from Mapbox
	geometry := make([]map[string]float64, 0, len(req.Waypoints))
	for _, wp := range req.Waypoints {
		geometry = append(geometry, map[string]float64{
			"latitude":  wp.Latitude,
			"longitude": wp.Longitude,
		})
	}

	response := gin.H{
		"geometry": geometry,
		"distance": totalDistance * 1000, // in meters
		"duration": totalDistance * 60,   // rough estimate in seconds
		"summary":  "Planned route",
		"steps":    []gin.H{}, // Would contain turn-by-turn instructions
	}

	c.JSON(http.StatusOK, response)
}

// CalculateMetrics calculates distance and time between two points
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

// Helper types and functions

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// ⭐ DEPRECATED: Az eredeti convertToJSONData függvény
func convertToJSONData(geometry []map[string]float64) models.JSONData {
	result := make(models.JSONData)
	for i, point := range geometry {
		result[fmt.Sprintf("%d", i)] = point
	}
	return result
}

func (rc *RouteController) calculateTotalDistance(waypoints []RouteWaypointRequestV) float64 {
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

	dLat := (lat2 - lat1) * (math.Pi / 180)
	dLon := (lon2 - lon1) * (math.Pi / 180)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*(math.Pi/180))*math.Cos(lat2*(math.Pi/180))*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func (rc *RouteController) getCurrentTimestamp() int64 {
	return int64(1000) // Mock timestamp - would use time.Now().Unix() in production
}
