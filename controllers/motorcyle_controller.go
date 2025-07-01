// File: /controllers/motorcycle_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"net/http"
)

type MotorcycleController struct {
	db *gorm.DB
}

func NewMotorcycleController(db *gorm.DB) *MotorcycleController {
	return &MotorcycleController{db: db}
}

type CreateMotorcycleRequest struct {
	Brand    string `json:"brand" binding:"required"`
	Model    string `json:"model" binding:"required"`
	Year     string `json:"year" binding:"required"`
	ImageURL string `json:"image_url"`
}

func (mc *MotorcycleController) GetMotorcycles(c *gin.Context) {
	userID := c.GetString("user_id")

	var motorcycles []models.Motorcycle
	if err := mc.db.Where("user_id = ?", userID).Find(&motorcycles).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch motorcycles"})
		return
	}

	c.JSON(http.StatusOK, motorcycles)
}

func (mc *MotorcycleController) CreateMotorcycle(c *gin.Context) {
	userID := c.GetString("user_id")

	var req CreateMotorcycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	motorcycle := models.Motorcycle{
		ID:       uuid.New().String(),
		UserID:   userID,
		Brand:    req.Brand,
		Model:    req.Model,
		Year:     req.Year,
		ImageURL: req.ImageURL,
	}

	if err := mc.db.Create(&motorcycle).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create motorcycle"})
		return
	}

	c.JSON(http.StatusCreated, motorcycle)
}

func (mc *MotorcycleController) UpdateMotorcycle(c *gin.Context) {
	userID := c.GetString("user_id")
	motorcycleID := c.Param("id")

	var motorcycle models.Motorcycle
	if err := mc.db.First(&motorcycle, "id = ? AND user_id = ?", motorcycleID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Motorcycle not found or access denied"})
		return
	}

	var req CreateMotorcycleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"brand":     req.Brand,
		"model":     req.Model,
		"year":      req.Year,
		"image_url": req.ImageURL,
	}

	if err := mc.db.Model(&motorcycle).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update motorcycle"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Motorcycle updated successfully"})
}

func (mc *MotorcycleController) DeleteMotorcycle(c *gin.Context) {
	userID := c.GetString("user_id")
	motorcycleID := c.Param("id")

	var motorcycle models.Motorcycle
	if err := mc.db.First(&motorcycle, "id = ? AND user_id = ?", motorcycleID, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Motorcycle not found or access denied"})
		return
	}

	if err := mc.db.Delete(&motorcycle).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete motorcycle"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Motorcycle deleted successfully"})
}
