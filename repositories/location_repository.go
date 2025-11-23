package repositories

import (
	"errors"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"time"
)

type LocationRepository struct {
	db *gorm.DB
}

func NewLocationRepository(db *gorm.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

// UpdateUserLocation updates or creates user's location
func (r *LocationRepository) UpdateUserLocation(location *models.UserLocation) error {
	var existingLocation models.UserLocation
	err := r.db.Where("user_id = ?", location.UserID).First(&existingLocation).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(location).Error
		}
		return err
	}

	return r.db.Model(&existingLocation).Updates(map[string]interface{}{
		"latitude":     location.Latitude,
		"longitude":    location.Longitude,
		"accuracy":     location.Accuracy,
		"is_available": location.IsAvailable,
		"is_online":    location.IsOnline,
		"last_seen":    location.LastSeen,
		"status":       location.Status,
		"updated_at":   time.Now(),
	}).Error
}

// GetUserLocation retrieves a user's current location
func (r *LocationRepository) GetUserLocation(userID string) (*models.UserLocation, error) {
	var location models.UserLocation
	err := r.db.Preload("User").Where("user_id = ?", userID).First(&location).Error
	if err != nil {
		return nil, err
	}
	return &location, nil
}

// GetVisibleLocations retrieves locations visible to the requesting user (friends only)
func (r *LocationRepository) GetVisibleLocations(userID string) ([]models.UserLocation, error) {
	var visibleLocations []models.UserLocation

	// Get user's friends
	friendIDs := r.getFriendIDs(userID)
	if len(friendIDs) == 0 {
		return visibleLocations, nil
	}

	// Get all locations of online friends
	var allLocations []models.UserLocation
	r.db.Preload("User").
		Where("user_id IN ?", friendIDs).
		Where("is_online = ?", true).
		Find(&allLocations)

	for _, loc := range allLocations {
		// Get this location owner's visibility settings
		ownerSettings, err := r.GetVisibilitySettings(loc.UserID)
		if err != nil {
			// No settings means default (friends only)
			visibleLocations = append(visibleLocations, loc)
			continue
		}

		// Check visibility based on settings
		switch ownerSettings.VisibilityMode {
		case "all":
			visibleLocations = append(visibleLocations, loc)
		case "friends":
			visibleLocations = append(visibleLocations, loc)
		case "custom":
			// Check if requesting user is in allowed list
			var allowed models.LocationVisibilityAllowed
			err := r.db.Where("owner_user_id = ? AND allowed_user_id = ?", loc.UserID, userID).First(&allowed).Error
			if err == nil {
				visibleLocations = append(visibleLocations, loc)
			}
		case "none":
			continue
		}
	}

	return visibleLocations, nil
}

// getFriendIDs returns all friend IDs for a user
func (r *LocationRepository) getFriendIDs(userID string) []string {
	var friendships []models.Friendship
	r.db.Where("user1_id = ? OR user2_id = ?", userID, userID).Find(&friendships)

	friendIDs := make([]string, 0, len(friendships))
	for _, friendship := range friendships {
		if friendship.User1ID == userID {
			friendIDs = append(friendIDs, friendship.User2ID)
		} else {
			friendIDs = append(friendIDs, friendship.User1ID)
		}
	}

	return friendIDs
}

// Visibility Settings Methods

// GetVisibilitySettings retrieves user's location visibility settings
func (r *LocationRepository) GetVisibilitySettings(userID string) (*models.LocationVisibilitySettings, error) {
	var settings models.LocationVisibilitySettings
	err := r.db.Preload("AllowedUsers").Where("user_id = ?", userID).First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// UpdateVisibilitySettings updates user's visibility settings
func (r *LocationRepository) UpdateVisibilitySettings(settings *models.LocationVisibilitySettings) error {
	var existing models.LocationVisibilitySettings
	err := r.db.Where("user_id = ?", settings.UserID).First(&existing).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(settings).Error
		}
		return err
	}

	return r.db.Model(&existing).Updates(map[string]interface{}{
		"visibility_mode": settings.VisibilityMode,
		"accuracy_level":  settings.AccuracyLevel,
		"updated_at":      time.Now(),
	}).Error
}

// SetAllowedUsers sets the custom allowed users list
func (r *LocationRepository) SetAllowedUsers(userID string, allowedUserIDs []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner_user_id = ?", userID).Delete(&models.LocationVisibilityAllowed{}).Error; err != nil {
			return err
		}

		for _, allowedUserID := range allowedUserIDs {
			allowed := models.LocationVisibilityAllowed{
				OwnerUserID:   userID,
				AllowedUserID: allowedUserID,
				CreatedAt:     time.Now(),
			}
			if err := tx.Create(&allowed).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// GetAllowedUsers retrieves the list of users allowed to see location
func (r *LocationRepository) GetAllowedUsers(userID string) ([]models.User, error) {
	var allowedRecords []models.LocationVisibilityAllowed
	err := r.db.Preload("AllowedUser").Where("owner_user_id = ?", userID).Find(&allowedRecords).Error
	if err != nil {
		return nil, err
	}

	users := make([]models.User, len(allowedRecords))
	for i, record := range allowedRecords {
		users[i] = record.AllowedUser
	}

	return users, nil
}

// IsUserAllowedToSeeLocation checks if a user can see another user's location
func (r *LocationRepository) IsUserAllowedToSeeLocation(ownerUserID, requestingUserID string) (bool, error) {
	// Check if they are friends
	if !r.areFriends(ownerUserID, requestingUserID) {
		return false, nil
	}

	// Get owner's settings
	settings, err := r.GetVisibilitySettings(ownerUserID)
	if err != nil {
		// Default to friends only if no settings
		return true, nil
	}

	switch settings.VisibilityMode {
	case "all":
		return true, nil
	case "friends":
		return true, nil
	case "custom":
		var allowed models.LocationVisibilityAllowed
		err := r.db.Where("owner_user_id = ? AND allowed_user_id = ?", ownerUserID, requestingUserID).First(&allowed).Error
		return err == nil, nil
	case "none":
		return false, nil
	default:
		return false, nil
	}
}

// areFriends checks if two users are friends
func (r *LocationRepository) areFriends(user1ID, user2ID string) bool {
	if user1ID > user2ID {
		user1ID, user2ID = user2ID, user1ID
	}

	var friendship models.Friendship
	err := r.db.Where("user1_id = ? AND user2_id = ?", user1ID, user2ID).First(&friendship).Error
	return err == nil
}

// CleanupOldLocations removes location records that haven't been updated recently
func (r *LocationRepository) CleanupOldLocations(olderThan time.Duration) error {
	cutoffTime := time.Now().Add(-olderThan)
	return r.db.Model(&models.UserLocation{}).
		Where("updated_at < ?", cutoffTime).
		Updates(map[string]interface{}{
			"is_online":    false,
			"is_available": false,
		}).Error
}

// GetFriendsWithLocation gets friends who have shared their location
func (r *LocationRepository) GetFriendsWithLocation(userID string) ([]models.UserLocation, error) {
	friendIDs := r.getFriendIDs(userID)
	if len(friendIDs) == 0 {
		return []models.UserLocation{}, nil
	}

	var locations []models.UserLocation
	err := r.db.Preload("User").
		Where("user_id IN ?", friendIDs).
		Where("is_online = ?", true).
		Find(&locations).Error

	return locations, err
}