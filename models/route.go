// File: /models/route.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Route represents a user's personal route (saved from route planning)
type Route struct {
	ID             string      `json:"id" gorm:"primaryKey;size:191"`
	UserID         string      `json:"user_id" gorm:"not null;size:191"`
	Name           string      `json:"name" gorm:"not null;size:255"`
	Description    string      `json:"description" gorm:"type:text"`
	TotalDistance  float64     `json:"total_distance"`  // km
	TotalElevation float64     `json:"total_elevation"` // m
	EstimatedTime  int         `json:"estimated_time"`  // in seconds
	Difficulty     string      `json:"difficulty" gorm:"size:50"`
	Tags           StringSlice `json:"tags" gorm:"type:json"`
	IsPublic       bool        `json:"is_public" gorm:"default:false"`
	TimesUsed      int         `json:"times_used" gorm:"default:0"`
	RouteGeometry  JSONData    `json:"route_geometry" gorm:"type:json"` // Detailed route points
	RouteSettings  JSONData    `json:"route_settings" gorm:"type:json"` // Route planning settings
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`

	// Relationships
	User      User            `json:"user" gorm:"foreignKey:UserID"`
	Waypoints []RouteWaypoint `json:"waypoints" gorm:"foreignKey:RouteID"`
}

// RouteWaypoint represents a waypoint in a user's route
type RouteWaypoint struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	RouteID     string  `json:"route_id" gorm:"not null;size:191"`
	Name        string  `json:"name" gorm:"size:255"`
	Description string  `json:"description" gorm:"type:text"`
	Latitude    float64 `json:"latitude" gorm:"not null"`
	Longitude   float64 `json:"longitude" gorm:"not null"`
	Order       int     `json:"order" gorm:"not null"`

	Route Route `json:"route" gorm:"foreignKey:RouteID"`
}

// SavedRoute represents a bookmark to another user's route
type SavedRoute struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"not null;size:191"`
	RouteID   string    `json:"route_id" gorm:"not null;size:191"`
	CreatedAt time.Time `json:"created_at"`

	User  User  `json:"user" gorm:"foreignKey:UserID"`
	Route Route `json:"route" gorm:"foreignKey:RouteID"`
}

// RouteResponse represents API response for routes with pagination
type RouteResponse struct {
	Routes     []Route `json:"routes"`
	Page       int     `json:"page"`
	Limit      int     `json:"limit"`
	Total      int64   `json:"total"`
	HasMore    bool    `json:"has_more"`
	TotalPages int     `json:"total_pages"`
}

// RouteWithInteractions represents a route with user interaction states
type RouteWithInteractions struct {
	Route
	UserInteractions RouteInteractions `json:"user_interactions"`
}

// RouteInteractions represents the current user's interactions with a route
type RouteInteractions struct {
	IsOwner      bool `json:"is_owner"`
	IsBookmarked bool `json:"is_bookmarked"`
	CanEdit      bool `json:"can_edit"`
}

// RouteMetrics represents calculated route metrics
type RouteMetrics struct {
	Distance float64 `json:"distance"` // in meters
	Duration float64 `json:"duration"` // in seconds
}

// RoutePlanRequest represents a route planning request
type RoutePlanRequest struct {
	Waypoints     []LatLng `json:"waypoints" binding:"required"`
	AvoidHighways bool     `json:"avoid_highways"`
	PreferWinding bool     `json:"prefer_winding"`
	Profile       string   `json:"profile"` // driving, cycling, walking
}

// RoutePlanResponse represents a route planning response
type RoutePlanResponse struct {
	Geometry []LatLng           `json:"geometry"`
	Distance float64            `json:"distance"` // in meters
	Duration float64            `json:"duration"` // in seconds
	Summary  string             `json:"summary"`
	Steps    []RouteInstruction `json:"steps"`
}

// RouteInstruction represents a turn-by-turn instruction
type RouteInstruction struct {
	Instruction string  `json:"instruction"`
	Distance    float64 `json:"distance"`
	Duration    float64 `json:"duration"`
	Maneuver    string  `json:"maneuver"`
}

// LatLng represents a latitude/longitude coordinate
type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Custom types for JSON handling
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, s)
}

type JSONData map[string]interface{}

func (j JSONData) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONData) Scan(value interface{}) error {
	if value == nil {
		*j = JSONData{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}

// Helper methods for Route model

// GetEstimatedDuration returns the estimated duration as a time.Duration
func (r *Route) GetEstimatedDuration() time.Duration {
	return time.Duration(r.EstimatedTime) * time.Second
}

// GetDifficultyLevel returns a numeric representation of difficulty
func (r *Route) GetDifficultyLevel() int {
	switch r.Difficulty {
	case "Easy":
		return 1
	case "Medium":
		return 2
	case "Hard":
		return 3
	default:
		return 1
	}
}

// IsAccessibleBy checks if a route is accessible by a given user
func (r *Route) IsAccessibleBy(userID string) bool {
	return r.UserID == userID || r.IsPublic
}

// CanBeEditedBy checks if a route can be edited by a given user
func (r *Route) CanBeEditedBy(userID string) bool {
	return r.UserID == userID
}

// IncrementUsage increments the times used counter
func (r *Route) IncrementUsage() {
	r.TimesUsed++
}

// GetRouteGeometryAsLatLng converts route geometry to LatLng slice
func (r *Route) GetRouteGeometryAsLatLng() []LatLng {
	var points []LatLng
	for _, point := range r.RouteGeometry {
		if pointMap, ok := point.(map[string]interface{}); ok {
			if lat, ok := pointMap["latitude"].(float64); ok {
				if lng, ok := pointMap["longitude"].(float64); ok {
					points = append(points, LatLng{
						Latitude:  lat,
						Longitude: lng,
					})
				}
			}
		}
	}
	return points
}

// GetWaypointsAsLatLng converts waypoints to LatLng slice
func (r *Route) GetWaypointsAsLatLng() []LatLng {
	var points []LatLng
	for _, waypoint := range r.Waypoints {
		points = append(points, LatLng{
			Latitude:  waypoint.Latitude,
			Longitude: waypoint.Longitude,
		})
	}
	return points
}

// HasTag checks if the route has a specific tag
func (r *Route) HasTag(tag string) bool {
	for _, t := range r.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// GetRouteSettings returns specific route setting value
func (r *Route) GetRouteSettings(key string) interface{} {
	if r.RouteSettings == nil {
		return nil
	}
	return r.RouteSettings[key]
}

// GetAvoidHighways returns avoid highways setting
func (r *Route) GetAvoidHighways() bool {
	if value := r.GetRouteSettings("avoid_highways"); value != nil {
		if boolValue, ok := value.(bool); ok {
			return boolValue
		}
	}
	return false
}

// GetPreferWindingRoads returns prefer winding roads setting
func (r *Route) GetPreferWindingRoads() bool {
	if value := r.GetRouteSettings("prefer_winding_roads"); value != nil {
		if boolValue, ok := value.(bool); ok {
			return boolValue
		}
	}
	return false
}

// Helper methods for RouteWaypoint

// ToLatLng converts waypoint to LatLng
func (rw *RouteWaypoint) ToLatLng() LatLng {
	return LatLng{
		Latitude:  rw.Latitude,
		Longitude: rw.Longitude,
	}
}

// IsStart checks if this is the starting waypoint
func (rw *RouteWaypoint) IsStart() bool {
	return rw.Order == 1
}

// IsEnd checks if this is the ending waypoint (assuming last order in route)
func (rw *RouteWaypoint) IsEnd(totalWaypoints int) bool {
	return rw.Order == totalWaypoints
}
