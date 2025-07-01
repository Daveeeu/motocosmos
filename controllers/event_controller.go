// File: /controllers/event_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
	"strconv"
	"time"
)

type EventController struct {
	db *gorm.DB
}

func NewEventController(db *gorm.DB) *EventController {
	return &EventController{db: db}
}

type CreateEventRequest struct {
	Title             string    `json:"title" binding:"required"`
	Description       string    `json:"description" binding:"required"`
	EventDate         time.Time `json:"event_date" binding:"required"`
	LocationName      string    `json:"location_name" binding:"required"`
	LocationLatitude  float64   `json:"location_latitude" binding:"required"`
	LocationLongitude float64   `json:"location_longitude" binding:"required"`
	LocationAddress   string    `json:"location_address"`
	Difficulty        string    `json:"difficulty" binding:"required"`
	EstimatedDistance float64   `json:"estimated_distance"`
	EstimatedDuration int       `json:"estimated_duration"`
	MaxParticipants   int       `json:"max_participants" binding:"required,min=2"`
	Tags              []string  `json:"tags"`
	RouteID           *string   `json:"route_id"`
	ImageUrls         []string  `json:"image_urls"`
}

func (ec *EventController) GetEvents(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset := (page - 1) * limit

	var events []models.CommunityEvent
	query := ec.db.Preload("Organizer").Where("event_date > ?", time.Now())

	if search := c.Query("search"); search != "" {
		query = query.Where("title LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if difficulty := c.Query("difficulty"); difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}

	if availableOnly := c.Query("available_only"); availableOnly == "true" {
		query = query.Where("is_full = ?", false)
	}

	if err := query.Order("event_date ASC").Offset(offset).Limit(limit).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	// Remove password from organizer data
	for i := range events {
		events[i].Organizer.Password = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"page":   page,
		"limit":  limit,
	})
}

func (ec *EventController) CreateEvent(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate event date is in the future
	if req.EventDate.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event date must be in the future"})
		return
	}

	// Get organizer info
	var organizer models.User
	if err := ec.db.First(&organizer, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	event := models.CommunityEvent{
		ID:                uuid.New().String(),
		Title:             req.Title,
		Description:       req.Description,
		OrganizerID:       userID,
		OrganizerName:     organizer.Name,
		OrganizerAvatar:   organizer.Name[:1], // First letter of name
		EventDate:         req.EventDate,
		LocationName:      req.LocationName,
		LocationLatitude:  req.LocationLatitude,
		LocationLongitude: req.LocationLongitude,
		LocationAddress:   req.LocationAddress,
		Difficulty:        req.Difficulty,
		EstimatedDistance: req.EstimatedDistance,
		EstimatedDuration: req.EstimatedDuration,
		MaxParticipants:   req.MaxParticipants,
		ParticipantsCount: 1, // Organizer is automatically a participant
		Tags:              models.StringSlice(req.Tags),
		RouteID:           req.RouteID,
		ImageUrls:         models.StringSlice(req.ImageUrls),
	}

	if err := ec.db.Create(&event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create event"})
		return
	}

	// Add organizer as participant
	participant := models.EventParticipant{
		EventID: event.ID,
		UserID:  userID,
	}
	ec.db.Create(&participant)

	c.JSON(http.StatusCreated, event)
}

func (ec *EventController) GetEvent(c *gin.Context) {
	eventID := c.Param("id")

	var event models.CommunityEvent
	if err := ec.db.Preload("Organizer").Preload("Participants").Preload("Route").
		First(&event, "id = ?", eventID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	event.Organizer.Password = ""
	c.JSON(http.StatusOK, event)
}

func (ec *EventController) UpdateEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	eventID := c.Param("id")

	var event models.CommunityEvent
	if err := ec.db.First(&event, "id = ? AND organizer_id = ?", eventID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found or access denied"})
		return
	}

	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate event date is in the future
	if req.EventDate.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event date must be in the future"})
		return
	}

	updates := map[string]interface{}{
		"title":              req.Title,
		"description":        req.Description,
		"event_date":         req.EventDate,
		"location_name":      req.LocationName,
		"location_latitude":  req.LocationLatitude,
		"location_longitude": req.LocationLongitude,
		"location_address":   req.LocationAddress,
		"difficulty":         req.Difficulty,
		"estimated_distance": req.EstimatedDistance,
		"estimated_duration": req.EstimatedDuration,
		"max_participants":   req.MaxParticipants,
		"tags":               models.StringSlice(req.Tags),
		"route_id":           req.RouteID,
		"image_urls":         models.StringSlice(req.ImageUrls),
	}

	// Check if reducing max participants would make event invalid
	if req.MaxParticipants < event.ParticipantsCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot reduce max participants below current count"})
		return
	}

	// Update is_full status
	updates["is_full"] = req.MaxParticipants <= event.ParticipantsCount

	if err := ec.db.Model(&event).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event updated successfully"})
}

func (ec *EventController) DeleteEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	eventID := c.Param("id")

	var event models.CommunityEvent
	if err := ec.db.First(&event, "id = ? AND organizer_id = ?", eventID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found or access denied"})
		return
	}

	// Delete participants first
	ec.db.Where("event_id = ?", eventID).Delete(&models.EventParticipant{})

	// Delete the event
	if err := ec.db.Delete(&event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete event"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event deleted successfully"})
}

func (ec *EventController) JoinEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	eventID := c.Param("id")

	var event models.CommunityEvent
	if err := ec.db.First(&event, "id = ?", eventID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	if event.IsFull {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event is full"})
		return
	}

	if event.EventDate.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot join past events"})
		return
	}

	// Check if already joined
	var existingParticipant models.EventParticipant
	if err := ec.db.Where("event_id = ? AND user_id = ?", eventID, userID).First(&existingParticipant).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Already joined this event"})
		return
	}

	// Join the event
	participant := models.EventParticipant{
		EventID: eventID,
		UserID:  userID,
	}

	if err := ec.db.Create(&participant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join event"})
		return
	}

	// Update participant count and check if full
	newCount := event.ParticipantsCount + 1
	isFull := newCount >= event.MaxParticipants

	ec.db.Model(&event).Updates(map[string]interface{}{
		"participants_count": newCount,
		"is_full":            isFull,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Successfully joined event"})
}

func (ec *EventController) LeaveEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	eventID := c.Param("id")

	var event models.CommunityEvent
	if err := ec.db.First(&event, "id = ?", eventID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Event not found"})
		return
	}

	if event.OrganizerID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Organizer cannot leave their own event"})
		return
	}

	var participant models.EventParticipant
	if err := ec.db.Where("event_id = ? AND user_id = ?", eventID, userID).First(&participant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not a participant of this event"})
		return
	}

	if err := ec.db.Delete(&participant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave event"})
		return
	}

	// Update participant count
	newCount := event.ParticipantsCount - 1
	ec.db.Model(&event).Updates(map[string]interface{}{
		"participants_count": newCount,
		"is_full":            false, // Can't be full if someone left
	})

	c.JSON(http.StatusOK, gin.H{"message": "Successfully left event"})
}

func (ec *EventController) LikeEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	eventID := c.Param("id")

	var participant models.EventParticipant
	if err := ec.db.Where("event_id = ? AND user_id = ?", eventID, userID).First(&participant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Must be a participant to like event"})
		return
	}

	if participant.IsLiked {
		c.JSON(http.StatusConflict, gin.H{"error": "Already liked this event"})
		return
	}

	// Update participant like status and event likes count
	ec.db.Model(&participant).Update("is_liked", true)
	ec.db.Model(&models.CommunityEvent{}).Where("id = ?", eventID).UpdateColumn("likes_count", gorm.Expr("likes_count + ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Event liked successfully"})
}

func (ec *EventController) UnlikeEvent(c *gin.Context) {
	userID := c.GetString("user_id")
	eventID := c.Param("id")

	var participant models.EventParticipant
	if err := ec.db.Where("event_id = ? AND user_id = ? AND is_liked = ?", eventID, userID, true).First(&participant).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Like not found"})
		return
	}

	// Update participant like status and event likes count
	ec.db.Model(&participant).Update("is_liked", false)
	ec.db.Model(&models.CommunityEvent{}).Where("id = ?", eventID).UpdateColumn("likes_count", gorm.Expr("likes_count - ?", 1))

	c.JSON(http.StatusOK, gin.H{"message": "Event unliked successfully"})
}

func (ec *EventController) GetJoinedEvents(c *gin.Context) {
	userID := c.GetString("user_id")

	var participants []models.EventParticipant
	if err := ec.db.Preload("Event").Preload("Event.Organizer").Where("user_id = ?", userID).Find(&participants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch joined events"})
		return
	}

	var events []models.CommunityEvent
	for _, participant := range participants {
		participant.Event.Organizer.Password = ""
		events = append(events, participant.Event)
	}

	c.JSON(http.StatusOK, events)
}

func (ec *EventController) GetCreatedEvents(c *gin.Context) {
	userID := c.GetString("user_id")

	var events []models.CommunityEvent
	if err := ec.db.Preload("Participants").Where("organizer_id = ?", userID).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch created events"})
		return
	}

	c.JSON(http.StatusOK, events)
}

func (ec *EventController) SearchEvents(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query required"})
		return
	}

	var events []models.CommunityEvent
	if err := ec.db.Preload("Organizer").Where("title LIKE ? OR description LIKE ? OR location_name LIKE ?",
		"%"+query+"%", "%"+query+"%", "%"+query+"%").
		Where("event_date > ?", time.Now()).
		Order("event_date ASC").Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search events"})
		return
	}

	// Remove password from organizer data
	for i := range events {
		events[i].Organizer.Password = ""
	}

	c.JSON(http.StatusOK, events)
}
