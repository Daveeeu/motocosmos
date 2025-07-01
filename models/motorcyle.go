// File: /models/motorcycle.go
package models

import (
	"time"
)

type Motorcycle struct {
	ID        string    `json:"id" gorm:"primaryKey;size:191"`
	UserID    string    `json:"user_id" gorm:"not null;size:191"`
	Brand     string    `json:"brand" gorm:"not null;size:100"`
	Model     string    `json:"model" gorm:"not null;size:100"`
	Year      string    `json:"year" gorm:"not null;size:4"`
	ImageURL  string    `json:"image_url" gorm:"size:500"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}
