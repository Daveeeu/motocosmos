// File: /services/location_service.go
package services

import (
	"errors"
	"math"
	"motocosmos-api/models"
	"motocosmos-api/repositories"
	"time"
)

type LocationService struct {
	locationRepo *repositories.LocationRepository
}

func NewLocationService(locationRepo *repositories.LocationRepository) *LocationService {
	return &LocationService{
		locationRepo: locationRepo,
	}
}

// UpdateLocation updates user's current location
func (s *LocationService) UpdateLocation(userID string, req models.UpdateLocationRequest, user *models.User) error {
	// Validate coordinates
	if !isValidLatitude(req.Latitude) || !isValidLongitude(req.Longitude) {
		return errors.New("invalid coordinates")
	}

	// Apply accuracy level if specified
	adjustedLat, adjustedLng := s.applyAccuracyLevel(req.Latitude, req.Longitude, req.AccuracyLevel)

	// Set default availability if not provided
	isAvailable := true
	if req.IsAvailable != nil {
		isAvailable = *req.IsAvailable
	}

	location := &models.UserLocation{
		ID:          userID + "_location",
		UserID:      userID,
		Username:    user.Name,
		Latitude:    adjustedLat,
		Longitude:   adjustedLng,
		Accuracy:    req.Accuracy,
		IsAvailable: isAvailable,
		IsOnline:    true,
		LastSeen:    time.Now(),
		Status:      req.Status,
		UpdatedAt:   time.Now(),
	}

	return s.locationRepo.UpdateUserLocation(location)
}

// GetLocatorData retrieves all visible users on the map for the requesting user
func (s *LocationService) GetLocatorData(userID string) (*models.LocatorResponse, error) {
	// Get current user's location to calculate distances
	currentLocation, err := s.locationRepo.GetUserLocation(userID)
	if err != nil {
		// User doesn't have location set, return empty but valid response
		return &models.LocatorResponse{
			Count: 0,
			Users: []models.LocatorUserResponse{},
		}, nil
	}

	// Get all visible locations
	locations, err := s.locationRepo.GetVisibleLocations(userID)
	if err != nil {
		return nil, err
	}

	// Build response
	users := make([]models.LocatorUserResponse, 0, len(locations))
	for _, loc := range locations {
		// Calculate distance
		distance := calculateDistance(
			currentLocation.Latitude,
			currentLocation.Longitude,
			loc.Latitude,
			loc.Longitude,
		)

		// Get owner's visibility settings to apply accuracy
		settings, err := s.locationRepo.GetVisibilitySettings(loc.UserID)
		adjustedLat, adjustedLng := loc.Latitude, loc.Longitude
		if err == nil {
			adjustedLat, adjustedLng = s.applyAccuracyLevel(loc.Latitude, loc.Longitude, settings.AccuracyLevel)
		}

		user := models.LocatorUserResponse{
			ID:          loc.UserID,
			Name:        loc.User.Name,
			AvatarURL:   "",
			Latitude:    adjustedLat,
			Longitude:   adjustedLng,
			IsAvailable: loc.IsAvailable,
			LastSeen:    loc.LastSeen,
			DistanceKm:  distance,
			Status:      loc.Status,
		}
		users = append(users, user)
	}

	return &models.LocatorResponse{
		Count: len(users),
		Users: users,
	}, nil
}

// UpdateVisibilitySettings updates user's location visibility settings
func (s *LocationService) UpdateVisibilitySettings(userID string, req models.UpdateVisibilityRequest) error {
	// Validate visibility mode
	validModes := map[string]bool{"all": true, "friends": true, "custom": true, "none": true}
	if !validModes[req.VisibilityMode] {
		return errors.New("invalid visibility mode")
	}

	// Validate accuracy level
	validAccuracy := map[string]bool{"precise": true, "approximate": true, "city": true}
	if !validAccuracy[req.AccuracyLevel] {
		return errors.New("invalid accuracy level")
	}

	settings := &models.LocationVisibilitySettings{
		UserID:         userID,
		VisibilityMode: req.VisibilityMode,
		AccuracyLevel:  req.AccuracyLevel,
		UpdatedAt:      time.Now(),
	}

	// Update settings
	if err := s.locationRepo.UpdateVisibilitySettings(settings); err != nil {
		return err
	}

	// If custom mode, update allowed users list
	if req.VisibilityMode == "custom" && req.AllowedUserIDs != nil {
		if err := s.locationRepo.SetAllowedUsers(userID, req.AllowedUserIDs); err != nil {
			return err
		}
	}

	return nil
}

// GetVisibilitySettings retrieves user's current visibility settings
func (s *LocationService) GetVisibilitySettings(userID string) (*models.VisibilitySettingsResponse, error) {
	settings, err := s.locationRepo.GetVisibilitySettings(userID)
	if err != nil {
		// Return default settings if not found
		return &models.VisibilitySettingsResponse{
			VisibilityMode: "friends",
			AccuracyLevel:  "precise",
			AllowedUsers:   []models.AllowedUserResponse{},
		}, nil
	}

	// Get allowed users if custom mode
	var allowedUsers []models.AllowedUserResponse
	if settings.VisibilityMode == "custom" {
		users, err := s.locationRepo.GetAllowedUsers(userID)
		if err == nil {
			for _, user := range users {
				allowedUsers = append(allowedUsers, models.AllowedUserResponse{
					ID:        user.ID,
					Name:      user.Name,
					AvatarURL: "",
				})
			}
		}
	}

	return &models.VisibilitySettingsResponse{
		VisibilityMode: settings.VisibilityMode,
		AccuracyLevel:  settings.AccuracyLevel,
		AllowedUsers:   allowedUsers,
	}, nil
}

// Helper functions

// applyAccuracyLevel adjusts coordinates based on privacy level
func (s *LocationService) applyAccuracyLevel(lat, lng float64, level string) (float64, float64) {
	switch level {
	case "precise":
		// Return exact coordinates
		return lat, lng
	case "approximate":
		// Round to ~100m precision (3 decimal places)
		return roundToDecimal(lat, 3), roundToDecimal(lng, 3)
	case "city":
		// Round to ~10km precision (1 decimal place)
		return roundToDecimal(lat, 1), roundToDecimal(lng, 1)
	default:
		return lat, lng
	}
}

// calculateDistance calculates distance between two points using Haversine formula
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371 // km

	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	distance := earthRadius * c

	return math.Round(distance*10) / 10 // Round to 1 decimal place
}

// roundToDecimal rounds a float to specified decimal places
func roundToDecimal(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

// isValidLatitude checks if latitude is valid
func isValidLatitude(lat float64) bool {
	return lat >= -90 && lat <= 90
}

// isValidLongitude checks if longitude is valid
func isValidLongitude(lng float64) bool {
	return lng >= -180 && lng <= 180
}

// CleanupInactiveLocations marks users as offline if they haven't updated recently
func (s *LocationService) CleanupInactiveLocations() error {
	// Mark users as offline if no update in last 15 minutes
	return s.locationRepo.CleanupOldLocations(15 * time.Minute)
}