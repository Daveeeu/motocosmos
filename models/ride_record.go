// File: /models/ride_record.go
package models

import (
	"time"
)

type RideRecord struct {
	ID             string      `json:"id" gorm:"primaryKey"`
	UserID         string      `json:"user_id" gorm:"not null"`
	MotorcycleID   string      `json:"motorcycle_id" gorm:"not null"`
	MotorcycleName string      `json:"motorcycle_name" gorm:"not null"`
	StartTime      time.Time   `json:"start_time" gorm:"not null"`
	EndTime        *time.Time  `json:"end_time"`
	Duration       int         `json:"duration"`        // in seconds
	Distance       float64     `json:"distance"`        // in km
	MaxSpeed       float64     `json:"max_speed"`       // in km/h
	AverageSpeed   float64     `json:"average_speed"`   // in km/h
	MaxAltitude    float64     `json:"max_altitude"`    // in meters
	TotalElevation float64     `json:"total_elevation"` // in meters
	PhotoUrls      StringSlice `json:"photo_urls" gorm:"type:json"`
	IsCompleted    bool        `json:"is_completed" gorm:"default:false"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`

	User        User         `json:"user" gorm:"foreignKey:UserID"`
	Motorcycle  Motorcycle   `json:"motorcycle" gorm:"foreignKey:MotorcycleID"`
	RoutePoints []RoutePoint `json:"route_points" gorm:"foreignKey:RideRecordID"`
}

type RoutePoint struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	RideRecordID string    `json:"ride_record_id" gorm:"not null"`
	Latitude     float64   `json:"latitude" gorm:"not null"`
	Longitude    float64   `json:"longitude" gorm:"not null"`
	Altitude     *float64  `json:"altitude"`
	Speed        *float64  `json:"speed"`
	Timestamp    time.Time `json:"timestamp" gorm:"not null"`

	RideRecord RideRecord `json:"ride_record" gorm:"foreignKey:RideRecordID"`
}
