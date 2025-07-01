// File: /controllers/calculator_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
)

type CalculatorController struct {
	db *gorm.DB
}

func NewCalculatorController(db *gorm.DB) *CalculatorController {
	return &CalculatorController{db: db}
}

type CalculateTripRequest struct {
	RoadLength             float64 `json:"road_length" binding:"required,gt=0"`
	AverageFuelPrice       float64 `json:"average_fuel_price" binding:"required,gt=0"`
	AverageFuelConsumption float64 `json:"average_fuel_consumption" binding:"required,gt=0"`
	OtherCosts             float64 `json:"other_costs"`
}

type SaveCalculationRequest struct {
	RouteName              string  `json:"route_name" binding:"required"`
	RoadLength             float64 `json:"road_length" binding:"required,gt=0"`
	AverageFuelPrice       float64 `json:"average_fuel_price" binding:"required,gt=0"`
	AverageFuelConsumption float64 `json:"average_fuel_consumption" binding:"required,gt=0"`
	OtherCosts             float64 `json:"other_costs"`
}

func (cc *CalculatorController) CalculateTrip(c *gin.Context) {
	var req CalculateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate input ranges
	if req.RoadLength > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Road length cannot exceed 10,000 km"})
		return
	}
	if req.AverageFuelPrice > 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fuel price cannot exceed 10 EUR/L"})
		return
	}
	if req.AverageFuelConsumption > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Fuel consumption cannot exceed 50 L/100km"})
		return
	}

	// Calculate fuel needed and costs
	fuelNeeded := (req.RoadLength * req.AverageFuelConsumption) / 100
	fuelCost := fuelNeeded * req.AverageFuelPrice
	totalCost := fuelCost + req.OtherCosts

	result := gin.H{
		"fuel_needed_liters": fuelNeeded,
		"fuel_cost":          fuelCost,
		"other_costs":        req.OtherCosts,
		"total_cost":         totalCost,
		"cost_per_km":        totalCost / req.RoadLength,
	}

	c.JSON(http.StatusOK, result)
}

func (cc *CalculatorController) SaveCalculation(c *gin.Context) {
	userID := c.GetString("user_id")

	var req SaveCalculationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Calculate total cost
	fuelNeeded := (req.RoadLength * req.AverageFuelConsumption) / 100
	fuelCost := fuelNeeded * req.AverageFuelPrice
	totalCost := fuelCost + req.OtherCosts

	calculation := models.TripCalculation{
		ID:                     uuid.New().String(),
		UserID:                 userID,
		RouteName:              req.RouteName,
		RoadLength:             req.RoadLength,
		AverageFuelPrice:       req.AverageFuelPrice,
		AverageFuelConsumption: req.AverageFuelConsumption,
		OtherCosts:             req.OtherCosts,
		TotalCost:              totalCost,
	}

	if err := cc.db.Create(&calculation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save calculation"})
		return
	}

	c.JSON(http.StatusCreated, calculation)
}

func (cc *CalculatorController) GetHistory(c *gin.Context) {
	userID := c.GetString("user_id")

	var calculations []models.TripCalculation
	if err := cc.db.Where("user_id = ?", userID).Order("created_at DESC").Limit(20).Find(&calculations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch calculation history"})
		return
	}

	c.JSON(http.StatusOK, calculations)
}

func (cc *CalculatorController) ClearHistory(c *gin.Context) {
	userID := c.GetString("user_id")

	if err := cc.db.Where("user_id = ?", userID).Delete(&models.TripCalculation{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear calculation history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Calculation history cleared successfully"})
}

func (cc *CalculatorController) GetFuelPrices(c *gin.Context) {
	fuelPrices := map[string]float64{
		"Germany":        1.65,
		"France":         1.72,
		"Italy":          1.68,
		"Spain":          1.45,
		"Netherlands":    1.78,
		"Austria":        1.52,
		"Switzerland":    1.85,
		"Czech Republic": 1.38,
		"Hungary":        1.45,
		"Poland":         1.35,
		"Slovakia":       1.42,
		"Slovenia":       1.48,
	}

	c.JSON(http.StatusOK, fuelPrices)
}

func (cc *CalculatorController) GetFuelConsumption(c *gin.Context) {
	fuelConsumption := map[string]float64{
		"Small Car (1.0-1.4L)":    6.5,
		"Compact Car (1.4-1.8L)":  7.5,
		"Mid-size Car (1.8-2.5L)": 8.5,
		"Large Car (2.5L+)":       10.5,
		"SUV/4WD":                 12.0,
		"Van/Minibus":             9.5,
		"Motorcycle 250cc":        3.5,
		"Motorcycle 500cc":        4.5,
		"Motorcycle 750cc":        5.5,
		"Motorcycle 1000cc+":      6.5,
		"Electric Car":            0.0,
	}

	c.JSON(http.StatusOK, fuelConsumption)
}
