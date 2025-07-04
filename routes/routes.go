// File: /routes/routes.go
package routes

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"motocosmos-api/config"
	"motocosmos-api/controllers"
	"motocosmos-api/middleware"
	"motocosmos-api/services"
)

func SetupRoutes(router *gin.Engine, db *gorm.DB, jwtSecret string) {
	cfg := config.Load()
	emailService := services.NewEmailService(cfg)

	// Initialize controllers in proper order - NotificationController first
	notificationController := controllers.NewNotificationController(db)
	authController := controllers.NewAuthController(db, jwtSecret, emailService)
	userController := controllers.NewUserController(db, notificationController)
	postController := controllers.NewPostController(db, notificationController)
	commentController := controllers.NewCommentController(db, notificationController)
	sharedRouteController := controllers.NewSharedRouteController(db, notificationController) // NEW

	// Add other controllers as needed

	// Global middleware
	router.Use(middleware.SecurityHeaders())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.RequestLogger())
	router.Use(middleware.RateLimit(100, 10)) // 100 requests per minute, burst of 10

	// API v1 group
	v1 := router.Group("/api/v1")
	v1.Use(middleware.ValidateJSON())
	v1.Use(middleware.PaginationDefaults())

	// Auth routes (public)
	auth := v1.Group("/auth")
	{
		auth.POST("/register", authController.Register)
		auth.POST("/login", authController.Login)
		auth.POST("/logout", authController.Logout)
		auth.POST("/send-verification", authController.SendVerificationCode)
		auth.POST("/verify-code", authController.VerifyCode)
		auth.POST("/reset-password", authController.ResetPassword)
	}

	// Protected routes (require authentication)
	protected := v1.Group("/")
	protected.Use(middleware.AuthMiddleware(jwtSecret))

	// User routes
	users := protected.Group("/users")
	{
		users.GET("/profile", userController.GetProfile)
		users.PUT("/profile", userController.UpdateProfile)
		users.GET("/statistics", userController.GetStatistics)
		users.POST("/follow/:user_id", userController.FollowUser)
		users.DELETE("/follow/:user_id", userController.UnfollowUser)
		users.GET("/followers", userController.GetFollowers)
		users.GET("/following", userController.GetFollowing)

		// Enhanced user endpoints
		users.GET("/following-status/:user_id", userController.GetFollowingStatus) // Check if following a user
		users.GET("/search", userController.SearchUsers)                           // Search users by name/handle
		users.GET("/handle/:handle", userController.GetUserByHandle)               // Get user by handle
	}

	// Notification routes
	notifications := protected.Group("/notifications")
	{
		notifications.GET("/", notificationController.GetNotifications)
		notifications.GET("/stats", notificationController.GetNotificationStats)
		notifications.PUT("/:id/read", notificationController.MarkAsRead)
		notifications.PUT("/read-all", notificationController.MarkAllAsRead)
		notifications.DELETE("/:id", notificationController.DeleteNotification)
	}

	// Post routes
	posts := protected.Group("/posts")
	{
		posts.GET("/", postController.GetPosts)
		posts.POST("/", postController.CreatePost)
		posts.GET("/feed", postController.GetFeed)
		posts.GET("/:id", postController.GetPost)
		posts.PUT("/:id", postController.UpdatePost)
		posts.DELETE("/:id", postController.DeletePost)
		posts.POST("/:id/like", postController.LikePost)
		posts.DELETE("/:id/unlike", postController.UnlikePost)
		posts.POST("/:id/share", postController.SharePost)
		posts.POST("/:id/comments", commentController.CreateComment)
		posts.GET("/:id/comments", commentController.GetComments)

		// Enhanced post endpoints
		posts.GET("/:id/interactions", postController.GetPostInteractions) // Get user interaction states for a post
		posts.POST("/:id/bookmark", postController.BookmarkPost)           // Bookmark a post
		posts.DELETE("/:id/bookmark", postController.UnbookmarkPost)       // Remove bookmark
		posts.GET("/bookmarked", postController.GetBookmarkedPosts)        // Get user's bookmarked posts
	}

	// NEW: Shared Routes - Public exploration of community routes
	sharedRoutes := protected.Group("/shared-routes")
	{
		// Core CRUD operations
		sharedRoutes.GET("/", sharedRouteController.GetSharedRoutes)         // Get all shared routes with pagination/filtering
		sharedRoutes.POST("/", sharedRouteController.CreateSharedRoute)      // Create a new shared route
		sharedRoutes.GET("/:id", sharedRouteController.GetSharedRoute)       // Get single shared route by ID
		sharedRoutes.PUT("/:id", sharedRouteController.UpdateSharedRoute)    // Update shared route (creator only)
		sharedRoutes.DELETE("/:id", sharedRouteController.DeleteSharedRoute) // Delete shared route (creator only)

		// Interaction endpoints
		sharedRoutes.POST("/:id/like", sharedRouteController.LikeSharedRoute)         // Toggle like on shared route
		sharedRoutes.POST("/:id/bookmark", sharedRouteController.BookmarkSharedRoute) // Toggle bookmark on shared route
		sharedRoutes.POST("/:id/download", sharedRouteController.DownloadSharedRoute) // Download/navigate to route

		// Collection endpoints
		sharedRoutes.GET("/bookmarked", sharedRouteController.GetBookmarkedRoutes) // Get user's bookmarked routes
		sharedRoutes.GET("/search", sharedRouteController.SearchSharedRoutes)      // Search routes by query

		// Utility endpoints
		sharedRoutes.GET("/tags/popular", sharedRouteController.GetPopularTags) // Get popular tags
		sharedRoutes.GET("/stats", sharedRouteController.GetSharedRouteStats)   // Get route statistics
	}

	// Motorcycle routes (if implemented)
	motorcycles := protected.Group("/motorcycles")
	{
		// Add motorcycle endpoints here when implemented
		_ = motorcycles // Prevent unused variable error
	}

	// Route routes (personal routes - different from shared routes)
	routes := protected.Group("/routes")
	{
		// Add route endpoints here when implemented
		_ = routes // Prevent unused variable error
	}

	// Event routes (if implemented)
	events := protected.Group("/events")
	{
		// Add event endpoints here when implemented
		_ = events // Prevent unused variable error
	}

	// Ride recording routes (if implemented)
	rides := protected.Group("/rides")
	{
		// Add ride recording endpoints here when implemented
		_ = rides // Prevent unused variable error
	}

	// Location routes (if implemented)
	locations := protected.Group("/locations")
	{
		// Add location endpoints here when implemented
		_ = locations // Prevent unused variable error
	}

	// Trip calculator routes (if implemented)
	calculator := protected.Group("/calculator")
	{
		// Add calculator endpoints here when implemented
		_ = calculator // Prevent unused variable error
	}

	// Health check endpoint (public)
	v1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "MotoCosmos API is running",
			"version": "1.0.0",
		})
	})

	// API documentation endpoint (public)
	v1.GET("/docs", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "API Documentation",
			"endpoints": gin.H{
				"auth": gin.H{
					"POST /auth/register":          "Register a new user",
					"POST /auth/login":             "Login user",
					"POST /auth/logout":            "Logout user",
					"POST /auth/send-verification": "Send verification code",
					"POST /auth/verify-code":       "Verify email code",
					"POST /auth/reset-password":    "Reset password",
				},
				"users": gin.H{
					"GET /users/profile":                   "Get current user profile",
					"PUT /users/profile":                   "Update user profile",
					"GET /users/statistics":                "Get user statistics",
					"POST /users/follow/:user_id":          "Follow a user",
					"DELETE /users/follow/:user_id":        "Unfollow a user",
					"GET /users/following-status/:user_id": "Check following status",
					"GET /users/followers":                 "Get user followers",
					"GET /users/following":                 "Get users being followed",
					"GET /users/search":                    "Search users",
					"GET /users/handle/:handle":            "Get user by handle",
				},
				"notifications": gin.H{
					"GET /notifications/":         "Get paginated notifications",
					"GET /notifications/stats":    "Get notification statistics",
					"PUT /notifications/:id/read": "Mark notification as read",
					"PUT /notifications/read-all": "Mark all notifications as read",
					"DELETE /notifications/:id":   "Delete notification",
				},
				"posts": gin.H{
					"GET /posts/":                 "Get all posts",
					"POST /posts/":                "Create a new post",
					"GET /posts/feed":             "Get personalized feed",
					"GET /posts/:id":              "Get single post",
					"PUT /posts/:id":              "Update post",
					"DELETE /posts/:id":           "Delete post",
					"POST /posts/:id/like":        "Like a post",
					"DELETE /posts/:id/unlike":    "Unlike a post",
					"POST /posts/:id/share":       "Share a post",
					"GET /posts/:id/interactions": "Get user interactions for post",
					"POST /posts/:id/bookmark":    "Bookmark a post",
					"DELETE /posts/:id/bookmark":  "Remove bookmark",
					"GET /posts/bookmarked":       "Get bookmarked posts",
				},
				"shared-routes": gin.H{
					"GET /shared-routes/":              "Get all shared routes with filtering",
					"POST /shared-routes/":             "Create a new shared route",
					"GET /shared-routes/:id":           "Get single shared route",
					"PUT /shared-routes/:id":           "Update shared route (creator only)",
					"DELETE /shared-routes/:id":        "Delete shared route (creator only)",
					"POST /shared-routes/:id/like":     "Toggle like on shared route",
					"POST /shared-routes/:id/bookmark": "Toggle bookmark on shared route",
					"POST /shared-routes/:id/download": "Download/navigate to shared route",
					"GET /shared-routes/bookmarked":    "Get user's bookmarked routes",
					"GET /shared-routes/search":        "Search shared routes",
					"GET /shared-routes/tags/popular":  "Get popular tags",
					"GET /shared-routes/stats":         "Get shared route statistics",
				},
			},
		})
	})
}

// CORS middleware for handling cross-origin requests
func SetupCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
