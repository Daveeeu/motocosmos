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
		EmailVerified: false,
	}

	if err := ac.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
		return
	}

	code, err := ac.emailService.SendVerificationEmail(user.Email, user.Name)
	if err != nil {
		fmt.Printf("Failed to send verification email: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	response := gin.H{"message": "Verification code sent to your email"}

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

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
		return
	}

	if !ac.emailService.VerifyCode(req.Email, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired verification code"})
		return
	}

	if err := ac.db.Model(&user).Update("email_verified", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify email"})
		return
	}

	go func() {
		if err := ac.emailService.SendWelcomeEmail(user.Email, user.Name); err != nil {
			fmt.Printf("Failed to send welcome email: %v\n", err)
		}
	}()

	token, err := ac.generateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	user.Password = ""
	user.EmailVerified = true

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified successfully",
		"token":   token,
		"user":    user,
	})
}

type ResendVerificationRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (ac *AuthController) ResendVerificationCode(c *gin.Context) {
	var req ResendVerificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already verified"})
		return
	}

	code, err := ac.emailService.SendVerificationEmail(user.Email, user.Name)
	if err != nil {
		fmt.Printf("Failed to resend verification email: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	response := gin.H{"message": "Verification code resent to your email"}

	if gin.Mode() == gin.DebugMode {
		response["debug_code"] = code
	}

	c.JSON(http.StatusOK, response)
}

// =====================================================
// PASSWORD RESET ENDPOINTS
// =====================================================

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func (ac *AuthController) SendPasswordResetCode(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Don't reveal if email exists for security
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a password reset code has been sent"})
		return
	}

	code, err := ac.emailService.SendPasswordResetEmail(user.Email, user.Name)
	if err != nil {
		fmt.Printf("Failed to send password reset email: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send password reset email"})
		return
	}

	response := gin.H{"message": "Password reset code sent to your email"}

	if gin.Mode() == gin.DebugMode {
		response["debug_code"] = code
	}

	c.JSON(http.StatusOK, response)
}

type ResetPasswordWithCodeRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

func (ac *AuthController) ResetPasswordWithCode(c *gin.Context) {
	var req ResetPasswordWithCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := ac.db.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if !ac.emailService.VerifyCode(req.Email, req.Code) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired reset code"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	if err := ac.db.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	go func() {
		if err := ac.emailService.SendPasswordChangedEmail(user.Email, user.Name); err != nil {
			fmt.Printf("Failed to send password changed email: %v\n", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "Password has been reset successfully. You can now login with your new password.",
	})
}

func (ac *AuthController) ResetPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ac.SendPasswordResetCode(c)
}

// Helper functions
func (ac *AuthController) generateJWT(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
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
			break
		}
		handle = fmt.Sprintf("%s_%d", baseHandle, counter)
		counter++
	}

	return handle
}