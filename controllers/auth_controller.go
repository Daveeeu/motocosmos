// File: /controllers/auth_controller.go
package controllers

import (
	"crypto/rand"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"motocosmos-api/models"
	"motocosmos-api/services"
	"net/http"
	"time"
)

type AuthController struct {
	db           *gorm.DB
	jwtSecret    string
	emailService *services.EmailService
}

func NewAuthController(db *gorm.DB, jwtSecret string, emailService *services.EmailService) *AuthController {
	return &AuthController{
		db:           db,
		jwtSecret:    jwtSecret,
		emailService: emailService,
	}
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Handle   string `json:"handle"` // Optional - will be generated if not provided
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

type RegisterResponse struct {
	Message string      `json:"message"`
	User    models.User `json:"user"`
}

func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := ac.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
		return
	}

	// Generate handle if not provided
	handle := req.Handle
	if handle == "" {
		handle = ac.generateUniqueHandle(req.Name)
	} else {
		// Check if provided handle is available
		if err := ac.db.Where("handle = ?", handle).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Handle already taken"})
			return
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user with EmailVerified = false
	user := models.User{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Handle:        handle,
		Email:         req.Email,
		Password:      string(hashedPassword),
		EmailVerified: false, // Explicitly set to false
	}

	if err := ac.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Send verification email
	_, err = ac.emailService.SendVerificationEmail(user.Email, user.Name)
	if err != nil {
		// Log error but don't fail registration
		fmt.Printf("Failed to send verification email: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	// Remove password from response
	user.Password = ""

	c.JSON(http.StatusCreated, RegisterResponse{
		Message: "Registration successful! Please check your email and enter the verification code to complete your account setup.",
		User:    user,
	})
}

func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erroasdr": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if email is verified
	if !user.EmailVerified {
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "Email not verified",
			"message": "Please verify your email before logging in. Check your email for the verification code.",
		})
		return
	}

	// Generate JWT token
	token, err := ac.generateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Remove password from response
	user.Password = ""

	c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
	})
}

func (ac *AuthController) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side
	// For enhanced security, you could implement a token blacklist
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

type VerificationCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (ac *AuthController) SendVerificationCode(c *gin.Context) {
	var req VerificationCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user exists
	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
		return
	}

	// Send verification email
	code, err := ac.emailService.SendVerificationEmail(user.Email, user.Name)
	if err != nil {
		fmt.Printf("Failed to send verification email: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	// For development/testing purposes, also return the code in response
	// Remove this in production!
	response := gin.H{"message": "Verification code sent to your email"}

	// Only include the code in development mode
	if gin.Mode() == gin.DebugMode {
		response["debug_code"] = code
	}

	c.JSON(http.StatusOK, response)
}

type VerifyCodeRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required"`
}

func (ac *AuthController) VerifyCode(c *gin.Context) {
	var req VerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
		return
	}

	// Verify the code using EmailService
	if !ac.emailService.VerifyCode(req.Email, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification code"})
		return
	}

	// Mark email as verified
	if err := ac.db.Model(&user).Update("email_verified", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	// Send welcome email
	go func() {
		if err := ac.emailService.SendWelcomeEmail(user.Email, user.Name); err != nil {
			fmt.Printf("Failed to send welcome email: %v\n", err)
		}
	}()

	// Generate JWT token
	token, err := ac.generateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Remove password from response
	user.Password = ""
	user.EmailVerified = true // Update the user object for response

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully",
		"token":   token,
		"user":    user,
	})
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// New endpoint to resend verification code
func (ac *AuthController) ResendVerificationCode(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if already verified
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
		return
	}

	// Send verification email
	code, err := ac.emailService.SendVerificationEmail(user.Email, user.Name)
	if err != nil {
		fmt.Printf("Failed to resend verification email: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	// For development/testing purposes, also return the code in response
	response := gin.H{"message": "Verification code resent to your email"}

	// Only include the code in development mode
	if gin.Mode() == gin.DebugMode {
		response["debug_code"] = code
	}

	c.JSON(http.StatusOK, response)
}

type ResetPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if user exists
	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if email exists or not for security
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent"})
		return
	}

	// In a real application, you would:
	// 1. Generate a secure reset token
	// 2. Store it in database with expiration
	// 3. Send reset email with the token

	c.JSON(http.StatusOK, gin.H{"message": "Password reset email sent"})
}

// Helper functions
func (ac *AuthController) generateJWT(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ac.jwtSecret))
}

func (ac *AuthController) generateVerificationCode() string {
	b := make([]byte, 2)
	rand.Read(b)
	return fmt.Sprintf("%04d", int(b[0])<<8+int(b[1])%10000)
}

func (ac *AuthController) generateUniqueHandle(baseName string) string {
	baseHandle := models.GenerateHandleFromName(baseName)
	handle := baseHandle
	counter := 1

	for {
		var existingUser models.User
		if err := ac.db.Where("handle = ?", handle).First(&existingUser).Error; err != nil {
			// Handle is available
			break
		}
		// Handle exists, try with number suffix
		handle = fmt.Sprintf("%s_%d", baseHandle, counter)
		counter++
	}

	return handle
}
