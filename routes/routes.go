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

func SetupRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config, emailService *services.EmailService) {
	// Controllers
	authController := controllers.NewAuthController(db, cfg.JWTSecret, emailService)
	userController := controllers.NewUserController(db)
	routeController := controllers.NewRouteController(db)
	eventController := controllers.NewEventController(db)
	postController := controllers.NewPostController(db)
	motorcycleController := controllers.NewMotorcycleController(db)
	rideController := controllers.NewRideController(db)
	locationController := controllers.NewLocationController(db)
	calculatorController := controllers.NewCalculatorController(db)

	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
			"status":  "healthy",
			"email":   "configured",
		})
	})

	// API version 1
	v1 := r.Group("/api/v1")

	// Auth routes (public)
	auth := v1.Group("/auth")
	{
		auth.POST("/login", authController.Login)
		auth.POST("/register", authController.Register)
		auth.POST("/logout", authController.Logout)
		auth.POST("/send-verification", authController.SendVerification)
		auth.POST("/verify-code", authController.VerifyCode)
		auth.POST("/reset-password", authController.ResetPassword)

		auth.GET("/debug/verification-code", authController.GetVerificationCode)

	}

	// Protected routes
	protected := v1.Group("/")
	protected.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	{
		// User routes
		users := protected.Group("/users")
		{
			users.GET("/profile", userController.GetProfile)
			users.PUT("/profile", userController.UpdateProfile)
			users.GET("/statistics", userController.GetStatistics)
			users.POST("/follow/:id", userController.FollowUser)
			users.DELETE("/follow/:id", userController.UnfollowUser)
			users.GET("/followers", userController.GetFollowers)
			users.GET("/following", userController.GetFollowing)
		}

		// Motorcycle routes
		motorcycles := protected.Group("/motorcycles")
		{
			motorcycles.GET("/", motorcycleController.GetMotorcycles)
			motorcycles.POST("/", motorcycleController.CreateMotorcycle)
			motorcycles.PUT("/:id", motorcycleController.UpdateMotorcycle)
			motorcycles.DELETE("/:id", motorcycleController.DeleteMotorcycle)
		}

		// Route routes
		routes := protected.Group("/routes")
		{
			routes.GET("/", routeController.GetRoutes)
			routes.POST("/", routeController.CreateRoute)
			routes.GET("/:id", routeController.GetRoute)
			routes.PUT("/:id", routeController.UpdateRoute)
			routes.DELETE("/:id", routeController.DeleteRoute)
			routes.POST("/:id/save", routeController.SaveRoute)
			routes.GET("/saved", routeController.GetSavedRoutes)
			routes.GET("/recommendations", routeController.GetRecommendations)
			routes.POST("/plan", routeController.PlanRoute)
			routes.POST("/calculate-metrics", routeController.CalculateMetrics)
		}

		// Community Event routes
		events := protected.Group("/events")
		{
			events.GET("/", eventController.GetEvents)
			events.POST("/", eventController.CreateEvent)
			events.GET("/:id", eventController.GetEvent)
			events.PUT("/:id", eventController.UpdateEvent)
			events.DELETE("/:id", eventController.DeleteEvent)
			events.POST("/:id/join", eventController.JoinEvent)
			events.DELETE("/:id/leave", eventController.LeaveEvent)
			events.POST("/:id/like", eventController.LikeEvent)
			events.DELETE("/:id/unlike", eventController.UnlikeEvent)
			events.GET("/joined", eventController.GetJoinedEvents)
			events.GET("/created", eventController.GetCreatedEvents)
			events.GET("/search", eventController.SearchEvents)
		}

		// Post routes
		posts := protected.Group("/posts")
		{
			posts.GET("/", postController.GetPosts)
			posts.POST("/", postController.CreatePost)
			posts.GET("/:id", postController.GetPost)
			posts.PUT("/:id", postController.UpdatePost)
			posts.DELETE("/:id", postController.DeletePost)
			posts.POST("/:id/like", postController.LikePost)
			posts.DELETE("/:id/unlike", postController.UnlikePost)
			posts.POST("/:id/share", postController.SharePost)
			posts.GET("/feed", postController.GetFeed)
		}

		// Ride Record routes
		rides := protected.Group("/rides")
		{
			rides.GET("/", rideController.GetRides)
			rides.POST("/start", rideController.StartRide)
			rides.PUT("/:id/pause", rideController.PauseRide)
			rides.PUT("/:id/resume", rideController.ResumeRide)
			rides.PUT("/:id/stop", rideController.StopRide)
			rides.GET("/:id", rideController.GetRide)
			rides.POST("/:id/share", rideController.ShareRide)
			rides.POST("/:id/route-points", rideController.AddRoutePoint)
		}

		// Location routes
		locations := protected.Group("/locations")
		{
			locations.PUT("/update", locationController.UpdateLocation)
			locations.GET("/nearby", locationController.GetNearbyUsers)
			locations.GET("/friends", locationController.GetFriends)
			locations.POST("/friend/:id", locationController.AddFriend)
			locations.DELETE("/friend/:id", locationController.RemoveFriend)
		}

		// Calculator routes
		calculator := protected.Group("/calculator")
		{
			calculator.POST("/calculate", calculatorController.CalculateTrip)
			calculator.POST("/save", calculatorController.SaveCalculation)
			calculator.GET("/history", calculatorController.GetHistory)
			calculator.DELETE("/history", calculatorController.ClearHistory)
			calculator.GET("/fuel-prices", calculatorController.GetFuelPrices)
			calculator.GET("/fuel-consumption", calculatorController.GetFuelConsumption)
		}
	}
}
