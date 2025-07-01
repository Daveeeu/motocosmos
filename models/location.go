// File: /models/location.go
package models

import (
	"time"
)

type UserLocation struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	UserID           string    `json:"user_id" gorm:"not null"`
	Username         string    `json:"username" gorm:"not null"`
	Latitude         float64   `json:"latitude" gorm:"not null"`
	Longitude        float64   `json:"longitude" gorm:"not null"`
	IsOnline         bool      `json:"is_online" gorm:"default:false"`
	LastSeen         time.Time `json:"last_seen"`
	Status           string    `json:"status"`
	IsLocationPublic bool      `json:"is_location_public" gorm:"default:false"`
	UpdatedAt        time.Time `json:"updated_at"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}
