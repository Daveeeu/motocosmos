// File: /controllers/auth_controller.go
package controllers

import (
	"github.com/gin-gonic/gin"
	_ "github.com/go-mail/mail/v2"
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

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func (ac *AuthController) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Check if email is verified
	if !user.EmailVerified {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Email not verified. Please verify your email address first.",
			"code":  "EMAIL_NOT_VERIFIED",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := ac.generateToken(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	user.Password = "" // Don't send password in response
	c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
	})
}

func (ac *AuthController) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "teszt": req})
		return
	}

	// Check if user already exists
	var existingUser models.User
	if err := ac.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Create user with email verification set to false
	user := models.User{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Email:         req.Email,
		Password:      string(hashedPassword),
		EmailVerified: false, // Email not verified yet
	}

	if err := ac.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Send verification email
	code, err := ac.emailService.SendVerificationEmail(user.Email, user.Name)
	if err != nil {
		// If email fails, delete the created user to maintain consistency
		ac.db.Delete(&user)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to send verification email. Please try again.",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":               "User registered successfully. Please check your email for verification code.",
		"user_id":               user.ID,
		"email":                 user.Email,
		"verification_required": true,
		"debug_code":            code, // Remove this in production
	})
}

func (ac *AuthController) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

func (ac *AuthController) SendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Verification code sent to your email",
		"email":      user.Email,
		"debug_code": code, // Remove this in production
	})
}

func (ac *AuthController) VerifyCode(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
		Code  string `json:"code" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify the code
	if !ac.emailService.VerifyCode(req.Email, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification code"})
		return
	}

	// Get user and update verification status
	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Update email verification status
	user.EmailVerified = true
	if err := ac.db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update verification status"})
		return
	}

	// Generate JWT token
	token, err := ac.generateToken(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Send welcome email
	go func() {
		if err := ac.emailService.SendWelcomeEmail(user.Email, user.Name); err != nil {
			println("⚠️ Failed to send welcome email:", err.Error())
		}
	}()

	user.Password = "" // Don't send password in response
	c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
	})
}

func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if user exists or not for security
		c.JSON(http.StatusOK, gin.H{"message": "If this email is registered, you will receive a password reset email"})
		return
	}

	// Generate reset token (in production, store this in database with expiration)
	resetToken := uuid.New().String()

	// Send password reset email
	err := ac.emailService.SendPasswordResetEmail(user.Email, user.Name, resetToken)
	if err != nil {
		println("⚠️ Failed to send password reset email:", err.Error())
	}

	c.JSON(http.StatusOK, gin.H{"message": "If this email is registered, you will receive a password reset email"})
}

// Debug endpoint to get verification code (remove in production)
func (ac *AuthController) GetVerificationCode(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email parameter required"})
		return
	}

	code := ac.emailService.GetVerificationCode(email)
	if code == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active verification code found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"email": email,
		"code":  code,
	})
}

func (ac *AuthController) generateToken(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(ac.jwtSecret))
}
