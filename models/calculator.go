// File: /models/calculator.go
package models

import (
	"time"
)

type TripCalculation struct {
	ID                     string    `json:"id" gorm:"primaryKey"`
	UserID                 string    `json:"user_id" gorm:"not null"`
	RouteName              string    `json:"route_name" gorm:"not null"`
	RoadLength             float64   `json:"road_length" gorm:"not null"`
	AverageFuelPrice       float64   `json:"average_fuel_price" gorm:"not null"`
	AverageFuelConsumption float64   `json:"average_fuel_consumption" gorm:"not null"`
	OtherCosts             float64   `json:"other_costs" gorm:"default:0"`
	TotalCost              float64   `json:"total_cost" gorm:"not null"`
	CreatedAt              time.Time `json:"created_at"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}
