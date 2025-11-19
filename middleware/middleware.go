// File: /middleware/middleware.go
package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"
	"strings"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
	Code    int    `json:"code"`
}

// ErrorHandler middleware for standardized error responses
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Handle any errors that occurred during request processing
		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Log the error for debugging
			fmt.Printf("Request error: %v\n", err.Error())

			// Return standardized error response
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "Internal server error",
				Message: "An unexpected error occurred",
				Code:    http.StatusInternalServerError,
			})
		}
	}
}

// RateLimiter implements a simple rate limiting mechanism
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mutex    sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int, burst int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		burst:    burst,
	}
}

// GetLimiter returns the rate limiter for a given key (IP address)
func (rl *RateLimiter) GetLimiter(key string) *rate.Limiter {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		limiter = rate.NewLimiter(rl.rate, rl.burst)
		rl.limiters[key] = limiter
	}

	return limiter
}

// CleanupLimiters removes old limiters to prevent memory leaks
func (rl *RateLimiter) CleanupLimiters() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	for key, limiter := range rl.limiters {
		if limiter.Allow() == false {
			// If limiter is at capacity, keep it
			continue
		}
		// Remove limiters that haven't been used recently
		delete(rl.limiters, key)
	}
}

// RateLimit middleware
func RateLimit(requestsPerMinute int, burst int) gin.HandlerFunc {
	rateLimiter := NewRateLimiter(requestsPerMinute, burst)

	// Start cleanup goroutine
	go func() {
		ticker := time.NewTicker(time.Minute * 10)
		defer ticker.Stop()

		for range ticker.C {
			rateLimiter.CleanupLimiters()
		}
	}()

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		limiter := rateLimiter.GetLimiter(clientIP)

		if !limiter.Allow() {
			// Rate limit exceeded
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

			c.JSON(http.StatusTooManyRequests, ErrorResponse{
				Error:   "Rate limit exceeded",
				Message: fmt.Sprintf("Too many requests. Limit: %d requests per minute", requestsPerMinute),
				Code:    http.StatusTooManyRequests,
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		remaining := burst - 1 // Approximate remaining requests
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Minute).Unix(), 10))

		c.Next()
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrorResponse represents validation error response
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Code    int               `json:"code"`
	Errors  []ValidationError `json:"validation_errors"`
}

// ValidateJSON middleware to ensure request has valid JSON content type
func ValidateJSON() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip validation for certain endpoints that handle file uploads
		skipPaths := []string{
			"/posts/upload-image",
			"/posts/upload-images",
			"/shared-routes/upload-image",
			"/users/upload-avatar",
		}

		// Check if current path should skip JSON validation
		for _, path := range skipPaths {
			if strings.Contains(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		// Skip validation for GET, DELETE, and OPTIONS requests
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		// For POST, PUT, PATCH - validate Content-Type
		contentType := c.GetHeader("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid content type",
				"message": "Content-Type must be application/json; charset=utf-8",
				"code":    http.StatusBadRequest,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequestLogger middleware for detailed request logging
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request duration
		latency := time.Since(start)

		// Get status code
		status := c.Writer.Status()

		// Build log message
		clientIP := c.ClientIP()
		method := c.Request.Method
		userAgent := c.Request.UserAgent()

		if raw != "" {
			path = path + "?" + raw
		}

		// Log format: [IP] METHOD PATH STATUS LATENCY USER_AGENT
		fmt.Printf("[%s] %s %s %d %v %s\n",
			clientIP,
			method,
			path,
			status,
			latency,
			userAgent,
		)
	}
}

// SecurityHeaders middleware adds security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Next()
	}
}

// PaginationDefaults middleware sets default pagination values
func PaginationDefaults() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set default page if not provided
		if c.Query("page") == "" {
			c.Request.URL.RawQuery += "&page=1"
		}

		// Set default limit if not provided
		if c.Query("limit") == "" {
			c.Request.URL.RawQuery += "&limit=10"
		}

		// Validate and limit max page size
		if limit := c.Query("limit"); limit != "" {
			if limitInt, err := strconv.Atoi(limit); err == nil && limitInt > 50 {
				c.Request.URL.RawQuery = c.Request.URL.RawQuery + "&limit=50"
			}
		}

		c.Next()
	}
}
