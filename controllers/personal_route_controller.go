// File: /controllers/personal_route_controller.go
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

type PersonalRouteController struct {
	db *gorm.DB
}

func NewPersonalRouteController(db *gorm.DB) *PersonalRouteController {
	return &PersonalRouteController{db: db}
}

type CreatePersonalRouteRequest struct {
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

type PersonalRouteResponse struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	Description       string               `json:"description"`
	TotalDistance     float64              `json:"total_distance"`
	TotalElevation    float64              `json:"total_elevation"`
	EstimatedDuration int                  `json:"estimated_duration"` // seconds
	Difficulty        string               `json:"difficulty"`
	Tags              []string             `json:"tags"`
	RoutePoints       []RoutePointResponse `json:"route_points"`
	IsPublic          bool                 `json:"is_public"`
	TimesUsed         int                  `json:"times_used"`
	CreatedAt         string               `json:"created_at"`
	UpdatedAt         string               `json:"updated_at"`
}

type RoutePointResponse struct {
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Elevation *float64 `json:"elevation,omitempty"`
}

type PaginatedPersonalRoutesResponse struct {
	Routes     []PersonalRouteResponse `json:"routes"`
	Page       int                     `json:"page"`
	Limit      int                     `json:"limit"`
	Total      int64                   `json:"total"`
	HasMore    bool                    `json:"has_more"`
	TotalPages int                     `json:"total_pages"`
}

// GetPersonalRoutes returns user's personal routes
func (prc *PersonalRouteController) GetPersonalRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset := (page - 1) * limit

	var routes []models.Route
	var total int64

	// Get total count
	prc.db.Model(&models.Route{}).Where("user_id = ?", userID).Count(&total)

	// Get paginated routes with waypoints
	if err := prc.db.Preload("Waypoints").Where("user_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch routes"})
		return
	}

	// Convert to response format
	routeResponses := make([]PersonalRouteResponse, 0, len(routes))
	for _, route := range routes {
		routeResponse := PersonalRouteResponse{
			ID:                route.ID,
			Name:              route.Name,
			Description:       route.Description,
			TotalDistance:     route.TotalDistance,
			TotalElevation:    prc.calculateTotalElevation(route.Waypoints),
			EstimatedDuration: route.EstimatedTime,
			Difficulty:        route.Difficulty,
			Tags:              []string(route.Tags),
			RoutePoints:       prc.waypointsToRoutePoints(route.Waypoints),
			IsPublic:          route.IsPublic,
			TimesUsed:         route.TimesUsed,
			CreatedAt:         route.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:         route.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
		routeResponses = append(routeResponses, routeResponse)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := PaginatedPersonalRoutesResponse{
		Routes:     routeResponses,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// CreatePersonalRoute creates a new personal route
func (prc *PersonalRouteController) CreatePersonalRoute(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreatePersonalRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"errorasd": err.Error()})
		return
	}

	// Calculate total distance from waypoints
	totalDistance := prc.calculateTotalDistanceFromWaypoints(req.Waypoints)

	// Create the route
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

	if err := prc.db.Create(&route).Error; err != nil {
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
		prc.db.Create(&waypoint)
	}

	// Load the complete route with waypoints for response
	prc.db.Preload("Waypoints").First(&route, "id = ?", route.ID)

	// Convert to response format
	routeResponse := PersonalRouteResponse{
		ID:                route.ID,
		Name:              route.Name,
		Description:       route.Description,
		TotalDistance:     route.TotalDistance,
		TotalElevation:    prc.calculateTotalElevation(route.Waypoints),
		EstimatedDuration: route.EstimatedTime,
		Difficulty:        route.Difficulty,
		Tags:              []string(route.Tags),
		RoutePoints:       prc.waypointsToRoutePoints(route.Waypoints),
		IsPublic:          route.IsPublic,
		TimesUsed:         route.TimesUsed,
		CreatedAt:         route.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         route.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	c.JSON(http.StatusCreated, routeResponse)
}

// GetPersonalRoute returns a single personal route by ID
func (prc *PersonalRouteController) GetPersonalRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.Route
	if err := prc.db.Preload("Waypoints").Where("id = ? AND user_id = ?", routeID, userID).First(&route).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
		return
	}

	// Increment times used
	prc.db.Model(&route).UpdateColumn("times_used", gorm.Expr("times_used + ?", 1))

	// Convert to response format
	routeResponse := PersonalRouteResponse{
		ID:                route.ID,
		Name:              route.Name,
		Description:       route.Description,
		TotalDistance:     route.TotalDistance,
		TotalElevation:    prc.calculateTotalElevation(route.Waypoints),
		EstimatedDuration: route.EstimatedTime,
		Difficulty:        route.Difficulty,
		Tags:              []string(route.Tags),
		RoutePoints:       prc.waypointsToRoutePoints(route.Waypoints),
		IsPublic:          route.IsPublic,
		TimesUsed:         route.TimesUsed + 1, // Include the increment
		CreatedAt:         route.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         route.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	c.JSON(http.StatusOK, routeResponse)
}

// Helper functions
func (prc *PersonalRouteController) calculateTotalDistanceFromWaypoints(waypoints []RouteWaypointRequest) float64 {
	if len(waypoints) < 2 {
		return 0
	}

	var totalDistance float64
	for i := 0; i < len(waypoints)-1; i++ {
		distance := prc.calculateDistance(
			waypoints[i].Latitude, waypoints[i].Longitude,
			waypoints[i+1].Latitude, waypoints[i+1].Longitude,
		)
		totalDistance += distance
	}

	return totalDistance
}

func (prc *PersonalRouteController) calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	// Haversine formula implementation
	const earthRadius = 6371 // km
	const deg2rad = math.Pi / 180

	dLat := (lat2 - lat1) * deg2rad
	dLon := (lon2 - lon1) * deg2rad

	a := 0.5 - 0.5*math.Cos(dLat) + math.Cos(lat1*deg2rad)*math.Cos(lat2*deg2rad)*(1-math.Cos(dLon))/2

	return earthRadius * 2 * math.Asin(math.Sqrt(a))
}

func (prc *PersonalRouteController) calculateTotalElevation(waypoints []models.RouteWaypoint) float64 {
	// Simple elevation calculation - in a real app you'd get elevation data from the waypoints
	// For now, return a calculated value based on distance and route complexity
	if len(waypoints) < 2 {
		return 0
	}

	// Simple estimation: more waypoints = more elevation changes
	return float64(len(waypoints)) * 50.0 // Rough estimate
}

func (prc *PersonalRouteController) waypointsToRoutePoints(waypoints []models.RouteWaypoint) []RoutePointResponse {
	routePoints := make([]RoutePointResponse, len(waypoints))

	for i, wp := range waypoints {
		elevation := float64(i * 10) // Simple elevation simulation
		routePoints[i] = RoutePointResponse{
			Latitude:  wp.Latitude,
			Longitude: wp.Longitude,
			Elevation: &elevation,
		}
	}

	return routePoints
}
