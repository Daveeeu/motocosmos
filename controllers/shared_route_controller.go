// File: /controllers/shared_route_controller.go
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
	"strings"
)

type SharedRouteController struct {
	db                     *gorm.DB
	notificationController *NotificationController
}

func NewSharedRouteController(db *gorm.DB, notificationController *NotificationController) *SharedRouteController {
	return &SharedRouteController{
		db:                     db,
		notificationController: notificationController,
	}
}

type CreateSharedRouteRequest struct {
	Title             string                    `json:"title" binding:"required"`
	Description       string                    `json:"description"`
	ImageUrls         []string                  `json:"image_urls"`
	RoutePoints       []models.SharedRoutePoint `json:"route_points" binding:"required"`
	TotalDistance     float64                   `json:"total_distance" binding:"required"`
	TotalElevation    float64                   `json:"total_elevation"`
	EstimatedDuration int                       `json:"estimated_duration" binding:"required"` // seconds
	Difficulty        string                    `json:"difficulty" binding:"required"`         // Easy, Medium, Hard
	Tags              []string                  `json:"tags"`
}

// GetSharedRoutes returns all shared routes with pagination and filtering
func (src *SharedRouteController) GetSharedRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var routes []models.SharedRoute
	var total int64

	// Build query
	query := src.db.Preload("Creator").Order("created_at DESC")

	// Apply filters
	if search := c.Query("search"); search != "" {
		searchPattern := "%" + search + "%"
		query = query.Where("title LIKE ? OR description LIKE ? OR creator_name LIKE ?",
			searchPattern, searchPattern, searchPattern)
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}

	if tag := c.Query("tag"); tag != "" {
		query = query.Where("JSON_CONTAINS(tags, ?)", fmt.Sprintf(`"%s"`, tag))
	}

	// Get total count
	query.Model(&models.SharedRoute{}).Count(&total)

	// Get paginated results
	if err := query.Offset(offset).Limit(limit).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch shared routes"})
		return
	}

	// Convert to SharedRouteWithInteractions
	routesWithInteractions := make([]models.SharedRouteWithInteractions, 0, len(routes))
	for _, route := range routes {
		routeWithInteractions := models.SharedRouteWithInteractions{
			SharedRoute:      route,
			UserInteractions: src.getUserInteractions(userID, route.ID),
		}
		// Remove password from creator data
		routeWithInteractions.SharedRoute.Creator.Password = ""
		routesWithInteractions = append(routesWithInteractions, routeWithInteractions)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.SharedRouteResponse{
		Routes:     routesWithInteractions,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetSharedRoute returns a single shared route by ID
func (src *SharedRouteController) GetSharedRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.SharedRoute
	if err := src.db.Preload("Creator").First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shared route not found"})
		return
	}

	routeWithInteractions := models.SharedRouteWithInteractions{
		SharedRoute:      route,
		UserInteractions: src.getUserInteractions(userID, route.ID),
	}
	routeWithInteractions.SharedRoute.Creator.Password = ""

	c.JSON(http.StatusOK, routeWithInteractions)
}

// CreateSharedRoute creates a new shared route
func (src *SharedRouteController) CreateSharedRoute(c *gin.Context) {
	fmt.Println("Ez egy log üzenet a konzolra.")
	userID := c.GetString("user_id")

	var req CreateSharedRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get creator info
	var creator models.User
	if err := src.db.First(&creator, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Convert RoutePoints to JSONData
	routePointsJSON := make(models.JSONData)
	for i, point := range req.RoutePoints {
		routePointsJSON[fmt.Sprintf("%d", i)] = map[string]interface{}{
			"latitude":  point.Latitude,
			"longitude": point.Longitude,
			"elevation": point.Elevation,
		}
	}

	// Create shared route
	route := models.SharedRoute{
		ID:                uuid.New().String(),
		Title:             req.Title,
		Description:       req.Description,
		CreatorID:         userID,
		CreatorName:       creator.Name,
		CreatorAvatar:     getInitials(creator.Name),
		ImageUrls:         models.StringSlice(req.ImageUrls),
		RoutePoints:       routePointsJSON,
		TotalDistance:     req.TotalDistance,
		TotalElevation:    req.TotalElevation,
		EstimatedDuration: req.EstimatedDuration,
		Difficulty:        req.Difficulty,
		Tags:              models.StringSlice(req.Tags),
	}

	if err := src.db.Create(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create shared route"})
		return
	}

	// Load the complete route with creator info
	src.db.Preload("Creator").First(&route, "id = ?", route.ID)
	route.Creator.Password = ""

	c.JSON(http.StatusCreated, route)
}

// UpdateSharedRoute updates a shared route (only by creator)
func (src *SharedRouteController) UpdateSharedRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.SharedRoute
	if err := src.db.First(&route, "id = ? AND creator_id = ?", routeID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shared route not found or access denied"})
		return
	}

	var req CreateSharedRouteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert RoutePoints to JSONData
	routePointsJSON := make(models.JSONData)
	for i, point := range req.RoutePoints {
		routePointsJSON[fmt.Sprintf("%d", i)] = map[string]interface{}{
			"latitude":  point.Latitude,
			"longitude": point.Longitude,
			"elevation": point.Elevation,
		}
	}

	updates := map[string]interface{}{
		"title":              req.Title,
		"description":        req.Description,
		"image_urls":         models.StringSlice(req.ImageUrls),
		"route_points":       routePointsJSON,
		"total_distance":     req.TotalDistance,
		"total_elevation":    req.TotalElevation,
		"estimated_duration": req.EstimatedDuration,
		"difficulty":         req.Difficulty,
		"tags":               models.StringSlice(req.Tags),
	}

	if err := src.db.Model(&route).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update shared route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shared route updated successfully"})
}

// DeleteSharedRoute deletes a shared route (only by creator)
func (src *SharedRouteController) DeleteSharedRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	var route models.SharedRoute
	if err := src.db.First(&route, "id = ? AND creator_id = ?", routeID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shared route not found or access denied"})
		return
	}

	// Delete likes and bookmarks first
	src.db.Where("route_id = ?", routeID).Delete(&models.SharedRouteLike{})
	src.db.Where("route_id = ?", routeID).Delete(&models.SharedRouteBookmark{})

	// Delete the route
	if err := src.db.Delete(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete shared route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Shared route deleted successfully"})
}

// LikeSharedRoute toggles like on a shared route
func (src *SharedRouteController) LikeSharedRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	// Check if route exists
	var route models.SharedRoute
	if err := src.db.First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shared route not found"})
		return
	}

	// Check if already liked
	var existingLike models.SharedRouteLike
	if err := src.db.Where("route_id = ? AND user_id = ?", routeID, userID).First(&existingLike).Error; err == nil {
		// Unlike: remove the like
		if err := src.db.Delete(&existingLike).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlike route"})
			return
		}

		// Update likes count
		src.db.Model(&route).UpdateColumn("likes_count", gorm.Expr("likes_count - ?", 1))

		c.JSON(http.StatusOK, gin.H{
			"message":  "Route unliked successfully",
			"is_liked": false,
		})
		return
	}

	// Like: create new like
	like := models.SharedRouteLike{
		RouteID: routeID,
		UserID:  userID,
	}

	if err := src.db.Create(&like).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to like route"})
		return
	}

	// Update likes count
	src.db.Model(&route).UpdateColumn("likes_count", gorm.Expr("likes_count + ?", 1))

	// Create notification for route like
	if err := src.notificationController.CreateLikeNotification(userID, route.CreatorID, routeID); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Failed to create like notification: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Route liked successfully",
		"is_liked": true,
	})
}

// BookmarkSharedRoute toggles bookmark on a shared route
func (src *SharedRouteController) BookmarkSharedRoute(c *gin.Context) {
	userID := c.GetString("user_id")
	routeID := c.Param("id")

	// Check if route exists
	var route models.SharedRoute
	if err := src.db.First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shared route not found"})
		return
	}

	// Check if already bookmarked
	var existingBookmark models.SharedRouteBookmark
	if err := src.db.Where("route_id = ? AND user_id = ?", routeID, userID).First(&existingBookmark).Error; err == nil {
		// Remove bookmark
		if err := src.db.Delete(&existingBookmark).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove bookmark"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":       "Bookmark removed successfully",
			"is_bookmarked": false,
		})
		return
	}

	// Create bookmark
	bookmark := models.SharedRouteBookmark{
		RouteID: routeID,
		UserID:  userID,
	}

	if err := src.db.Create(&bookmark).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bookmark route"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Route bookmarked successfully",
		"is_bookmarked": true,
	})
}

// DownloadSharedRoute increments download count for a route
func (src *SharedRouteController) DownloadSharedRoute(c *gin.Context) {
	routeID := c.Param("id")

	var route models.SharedRoute
	if err := src.db.First(&route, "id = ?", routeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Shared route not found"})
		return
	}

	// Update downloads count
	src.db.Model(&route).UpdateColumn("downloads_count", gorm.Expr("downloads_count + ?", 1))

	// In the future, this would redirect to a route detail/navigation screen
	// For now, just return success with route data
	c.JSON(http.StatusOK, gin.H{
		"message":      "Route download initiated",
		"route_id":     routeID,
		"redirect_url": fmt.Sprintf("/routes/%s/navigate", routeID), // Future navigation screen
	})
}

// GetBookmarkedRoutes returns user's bookmarked routes
func (src *SharedRouteController) GetBookmarkedRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var bookmarks []models.SharedRouteBookmark
	var total int64

	// Get total count
	src.db.Model(&models.SharedRouteBookmark{}).Where("user_id = ?", userID).Count(&total)

	if err := src.db.Preload("Route.Creator").Where("user_id = ?", userID).
		Order("created_at DESC").Offset(offset).Limit(limit).Find(&bookmarks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch bookmarked routes"})
		return
	}

	// Convert to SharedRouteWithInteractions
	routesWithInteractions := make([]models.SharedRouteWithInteractions, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		routeWithInteractions := models.SharedRouteWithInteractions{
			SharedRoute:      bookmark.Route,
			UserInteractions: src.getUserInteractions(userID, bookmark.Route.ID),
		}
		routeWithInteractions.SharedRoute.Creator.Password = ""
		routesWithInteractions = append(routesWithInteractions, routeWithInteractions)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.SharedRouteResponse{
		Routes:     routesWithInteractions,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// SearchSharedRoutes searches routes by title, description, creator name, or tags
func (src *SharedRouteController) SearchSharedRoutes(c *gin.Context) {
	userID := c.GetString("user_id")
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query is required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var routes []models.SharedRoute
	var total int64

	searchPattern := "%" + query + "%"

	// Complex search across multiple fields
	dbQuery := src.db.Preload("Creator").Where(
		"title LIKE ? OR description LIKE ? OR creator_name LIKE ? OR JSON_SEARCH(tags, 'one', ?) IS NOT NULL",
		searchPattern, searchPattern, searchPattern, query,
	).Order("created_at DESC")

	// Get total count
	dbQuery.Model(&models.SharedRoute{}).Count(&total)

	// Get paginated results
	if err := dbQuery.Offset(offset).Limit(limit).Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search shared routes"})
		return
	}

	// Convert to SharedRouteWithInteractions
	routesWithInteractions := make([]models.SharedRouteWithInteractions, 0, len(routes))
	for _, route := range routes {
		routeWithInteractions := models.SharedRouteWithInteractions{
			SharedRoute:      route,
			UserInteractions: src.getUserInteractions(userID, route.ID),
		}
		routeWithInteractions.SharedRoute.Creator.Password = ""
		routesWithInteractions = append(routesWithInteractions, routeWithInteractions)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	hasMore := page < totalPages

	response := models.SharedRouteResponse{
		Routes:     routesWithInteractions,
		Page:       page,
		Limit:      limit,
		Total:      total,
		HasMore:    hasMore,
		TotalPages: totalPages,
	}

	c.JSON(http.StatusOK, response)
}

// GetPopularTags returns the most popular tags used in shared routes
func (src *SharedRouteController) GetPopularTags(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// This is a simplified approach - in production you might want to use a more efficient query
	var routes []models.SharedRoute
	if err := src.db.Select("tags").Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tags"})
		return
	}

	// Count tag occurrences
	tagCounts := make(map[string]int)
	for _, route := range routes {
		for _, tag := range route.Tags {
			tagCounts[tag]++
		}
	}

	// Convert to sorted slice
	var tagInfos []models.TagInfo
	for tag, count := range tagCounts {
		tagInfos = append(tagInfos, models.TagInfo{
			Name:  tag,
			Count: count,
		})
	}

	// Sort by count (you might want to implement proper sorting)
	// For now, just return up to limit
	if len(tagInfos) > limit {
		tagInfos = tagInfos[:limit]
	}

	response := models.PopularTagsResponse{
		Tags: tagInfos,
	}

	c.JSON(http.StatusOK, response)
}

// GetSharedRouteStats returns statistics about shared routes
func (src *SharedRouteController) GetSharedRouteStats(c *gin.Context) {
	var stats models.SharedRouteStats

	// Get total routes
	src.db.Model(&models.SharedRoute{}).Count(&stats.TotalRoutes)

	// Get total distance
	var totalDistance float64
	src.db.Model(&models.SharedRoute{}).Select("SUM(total_distance)").Scan(&totalDistance)
	stats.TotalDistance = totalDistance

	// Get total downloads
	var totalDownloads int64
	src.db.Model(&models.SharedRoute{}).Select("SUM(downloads_count)").Scan(&totalDownloads)
	stats.TotalDownloads = totalDownloads

	// Get popular tags (simplified)
	limit := 5
	var routes []models.SharedRoute
	src.db.Select("tags").Find(&routes)

	tagCounts := make(map[string]int)
	for _, route := range routes {
		for _, tag := range route.Tags {
			tagCounts[tag]++
		}
	}

	var popularTags []models.TagInfo
	for tag, count := range tagCounts {
		popularTags = append(popularTags, models.TagInfo{
			Name:  tag,
			Count: count,
		})
		if len(popularTags) >= limit {
			break
		}
	}
	stats.PopularTags = popularTags

	c.JSON(http.StatusOK, stats)
}

// Helper function to get user interaction states
func (src *SharedRouteController) getUserInteractions(userID, routeID string) models.SharedRouteInteractions {
	var interactions models.SharedRouteInteractions

	// Check if liked
	var like models.SharedRouteLike
	if err := src.db.Where("route_id = ? AND user_id = ?", routeID, userID).First(&like).Error; err == nil {
		interactions.IsLiked = true
	}

	// Check if bookmarked
	var bookmark models.SharedRouteBookmark
	if err := src.db.Where("route_id = ? AND user_id = ?", routeID, userID).First(&bookmark).Error; err == nil {
		interactions.IsBookmarked = true
	}

	return interactions
}

// Helper function to get initials from name
func getInitials(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return "U"
	}
	if len(words) == 1 {
		return strings.ToUpper(string(words[0][0]))
	}
	return strings.ToUpper(string(words[0][0]) + string(words[1][0]))
}
