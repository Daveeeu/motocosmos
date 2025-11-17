package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"io"
	"motocosmos-api/models"
	"net/http"
	"time"
)

type SocialAuthController struct {
	db        *gorm.DB
	jwtSecret string
}

func NewSocialAuthController(db *gorm.DB, jwtSecret string) *SocialAuthController {
	return &SocialAuthController{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

// =========================
// GOOGLE AUTHENTICATION
// =========================

type GoogleLoginRequest struct {
	IDToken string `json:"id_token" binding:"required"`
}

type GoogleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

func (sac *SocialAuthController) GoogleLogin(c *gin.Context) {
	var req GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify Google ID token
	userInfo, err := sac.verifyGoogleToken(req.IDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Google token"})
		return
	}

	// Find or create user
	user, isNewUser, err := sac.findOrCreateSocialUser(userInfo.Email, userInfo.Name, &userInfo.Picture, "google")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
		return
	}

	// Generate JWT token
	token, err := sac.generateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user":        user,
		"is_new_user": isNewUser,
	})
}

func (sac *SocialAuthController) verifyGoogleToken(idToken string) (*GoogleUserInfo, error) {
	resp, err := http.Get("https://www.googleapis.com/oauth2/v3/tokeninfo?id_token=" + idToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo GoogleUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// =========================
// APPLE AUTHENTICATION
// =========================

type AppleLoginRequest struct {
	IDToken           string  `json:"id_token" binding:"required"`
	AuthorizationCode string  `json:"authorization_code" binding:"required"`
	User              *string `json:"user"` // Only on first login
}

type AppleUserInfo struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name,omitempty"`
}

func (sac *SocialAuthController) AppleLogin(c *gin.Context) {
	var req AppleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify Apple ID token
	userInfo, err := sac.verifyAppleToken(req.IDToken, req.User)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Apple token"})
		return
	}

	// Find or create user
	user, isNewUser, err := sac.findOrCreateSocialUser(userInfo.Email, userInfo.Name, nil, "apple")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
		return
	}

	// Generate JWT token
	token, err := sac.generateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user":        user,
		"is_new_user": isNewUser,
	})
}

func (sac *SocialAuthController) verifyAppleToken(idToken string, userJSON *string) (*AppleUserInfo, error) {
	// Parse JWT token
	token, _, err := new(jwt.Parser).ParseUnverified(idToken, jwt.MapClaims{})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	email, _ := claims["email"].(string)
	emailVerified, _ := claims["email_verified"].(bool)

	userInfo := &AppleUserInfo{
		Email:         email,
		EmailVerified: emailVerified,
		Name:          email,
	}

	// Parse user info if provided
	if userJSON != nil && *userJSON != "" {
		var userData map[string]interface{}
		if err := json.Unmarshal([]byte(*userJSON), &userData); err == nil {
			if name, ok := userData["name"].(map[string]interface{}); ok {
				firstName, _ := name["firstName"].(string)
				lastName, _ := name["lastName"].(string)
				if firstName != "" || lastName != "" {
					userInfo.Name = firstName + " " + lastName
				}
			}
		}
	}

	return userInfo, nil
}

// =========================
// FACEBOOK AUTHENTICATION
// =========================

type FacebookLoginRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

type FacebookUserInfo struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

func (sac *SocialAuthController) FacebookLogin(c *gin.Context) {
	var req FacebookLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify Facebook access token
	userInfo, err := sac.verifyFacebookToken(req.AccessToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Facebook token"})
		return
	}

	// Find or create user
	avatarURL := userInfo.Picture.Data.URL
	user, isNewUser, err := sac.findOrCreateSocialUser(userInfo.Email, userInfo.Name, &avatarURL, "facebook")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
		return
	}

	// Generate JWT token
	token, err := sac.generateJWT(user.ID, user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	user.Password = ""

	c.JSON(http.StatusOK, gin.H{
		"token":       token,
		"user":        user,
		"is_new_user": isNewUser,
	})
}

func (sac *SocialAuthController) verifyFacebookToken(accessToken string) (*FacebookUserInfo, error) {
	resp, err := http.Get(fmt.Sprintf("https://graph.facebook.com/me?fields=id,name,email,picture&access_token=%s", accessToken))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid token")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var userInfo FacebookUserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// =========================
// HELPER FUNCTIONS
// =========================

func (sac *SocialAuthController) findOrCreateSocialUser(email, name string, avatar *string, provider string) (models.User, bool, error) {
	var user models.User
	isNewUser := false

	// Try to find existing user by email
	err := sac.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new user
			handle := sac.generateUniqueHandle(name)
			user = models.User{
				ID:            uuid.New().String(),
				Name:          name,
				Handle:        handle,
				Email:         email,
				Password:      uuid.New().String(), // Random password
				EmailVerified: true,                // Social login emails are verified
				Avatar:        avatar,
			}

			if err := sac.db.Create(&user).Error; err != nil {
				return user, false, err
			}

			isNewUser = true
		} else {
			return user, false, err
		}
	} else {
		// Update avatar if not set
		if avatar != nil && user.Avatar == nil {
			user.Avatar = avatar
			sac.db.Save(&user)
		}
	}

	return user, isNewUser, nil
}

func (sac *SocialAuthController) generateUniqueHandle(baseName string) string {
	baseHandle := models.GenerateHandleFromName(baseName)
	handle := baseHandle
	counter := 1

	for {
		var existingUser models.User
		if err := sac.db.Where("handle = ?", handle).First(&existingUser).Error; err != nil {
			break
		}
		handle = fmt.Sprintf("%s_%d", baseHandle, counter)
		counter++
	}

	return handle
}

func (sac *SocialAuthController) generateJWT(userID, email string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(sac.jwtSecret))
}