// File: /models/route.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

type Route struct {
	ID            string      `json:"id" gorm:"primaryKey"`
	UserID        string      `json:"user_id" gorm:"not null"`
	Name          string      `json:"name" gorm:"not null"`
	Description   string      `json:"description"`
	TotalDistance float64     `json:"total_distance"`
	EstimatedTime int         `json:"estimated_time"` // in seconds
	Difficulty    string      `json:"difficulty"`
	Tags          StringSlice `json:"tags" gorm:"type:json"`
	IsPublic      bool        `json:"is_public" gorm:"default:false"`
	TimesUsed     int         `json:"times_used" gorm:"default:0"`
	RouteGeometry JSONData    `json:"route_geometry" gorm:"type:json"`
	RouteSettings JSONData    `json:"route_settings" gorm:"type:json"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`

	User      User            `json:"user" gorm:"foreignKey:UserID"`
	Waypoints []RouteWaypoint `json:"waypoints" gorm:"foreignKey:RouteID"`
}

type RouteWaypoint struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	RouteID     string  `json:"route_id" gorm:"not null"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Latitude    float64 `json:"latitude" gorm:"not null"`
	Longitude   float64 `json:"longitude" gorm:"not null"`
	Order       int     `json:"order" gorm:"not null"`

	Route Route `json:"route" gorm:"foreignKey:RouteID"`
}

type SavedRoute struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    string    `json:"user_id" gorm:"not null"`
	RouteID   string    `json:"route_id" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`

	User  User  `json:"user" gorm:"foreignKey:UserID"`
	Route Route `json:"route" gorm:"foreignKey:RouteID"`
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
