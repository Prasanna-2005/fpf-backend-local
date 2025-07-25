// Package server provides HTTP server setup and route configuration
package server

import (
	"form_pf_backend/config"
	"form_pf_backend/ctrl"

	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// SetupRoutes configures and returns a Gin engine with routes and middleware
func SetupRoutes(cfg *config.RouteConfig) *gin.Engine {
	r := gin.Default()

	// Configure CORS if enabled
	if cfg.EnableCORS {
		corsConfig := cors.Config{
			AllowOrigins:     cfg.CORSOrigins,
			AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}
		r.Use(cors.New(corsConfig))
	}

	setupAPIRoutes(r, cfg.APIVersion)

	return r
}

// setupAPIRoutes creates the main API route group
func setupAPIRoutes(r *gin.Engine, version string) {
	api := r.Group("/api/" + version)
	setupAuthRoutes(api)
}

// setupAuthRoutes configures authentication-related routes
func setupAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", ctrl.Login)
	auth.POST("/verify_otp", ctrl.VerifyOTP)
	auth.POST("/resend_otp", ctrl.ResendOTP)
	auth.POST("/access_token", ctrl.NewAccessToken)

}

// SetupRoutesWithDefaults loads config from file and sets up routes with default configuration
func SetupRoutesWithDefaults() *gin.Engine {
	config, err := config.LoadConfig("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return SetupRoutes(config)
}
