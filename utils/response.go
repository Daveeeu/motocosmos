// File: /utils/response.go
package utils

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code,omitempty"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}

func SendError(c *gin.Context, status int, err string) {
	c.JSON(status, ErrorResponse{
		Error: err,
		Code:  status,
	})
}

func SendValidationError(c *gin.Context, err string) {
	c.JSON(http.StatusBadRequest, ErrorResponse{
		Error:   "Validation failed",
		Message: err,
		Code:    http.StatusBadRequest,
	})
}

func SendSuccess(c *gin.Context, message string, data interface{}) {
	response := SuccessResponse{
		Message: message,
	}
	if data != nil {
		response.Data = data
	}
	c.JSON(http.StatusOK, response)
}

func SendCreated(c *gin.Context, message string, data interface{}) {
	response := SuccessResponse{
		Message: message,
		Data:    data,
	}
	c.JSON(http.StatusCreated, response)
}

func SendPaginated(c *gin.Context, data interface{}, page, limit int, total int64) {
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	c.JSON(http.StatusOK, PaginatedResponse{
		Data:       data,
		Page:       page,
		Limit:      limit,
		Total:      total,
		TotalPages: totalPages,
	})
}
