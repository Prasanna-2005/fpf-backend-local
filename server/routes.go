// Package server provides HTTP server setup and route configuration
package server

import (
	"form_pf_backend/config"
	"form_pf_backend/ctrl"
	"form_pf_backend/middleware"

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
	setupProfileRoutes(api)
	setupAdminRoutes(api)
}

// setupAuthRoutes configures authentication-related routes
func setupAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", ctrl.Login)
	auth.POST("/verify_otp", ctrl.VerifyOTP)
	auth.POST("/resend_otp", ctrl.ResendOTP)
	auth.POST("/logout", ctrl.Logout)
	auth.POST("/access_token", ctrl.NewAccessToken)
}

func setupProfileRoutes(api *gin.RouterGroup) {
	prof := api.Group("/profile")
	prof.Use(middleware.Authenticate(""))
	prof.POST("/reset", ctrl.UpdatePasswd)
}

func setupFundRoutes(api *gin.RouterGroup) {
	fund := api.Group("/fund")
	fund.Use(middleware.Authenticate(""))
	fund.GET("/fund_details", ctrl.GetFunds)
}

func setupAdminRoutes(api *gin.RouterGroup) {
	admin := api.Group("/admin")
	admin.Use(middleware.Authenticate("admin"))
	admin.GET("/users", ctrl.GetUsers)
	admin.POST("/create_user", ctrl.NewUser)
	// admin.DELETE("/delete_user/:user_id", ctrl.DeleteUser)

}

// func setupFileRoutes(api *gin.RouterGroup){
// 	file:=api.Group("/files")
// 	files.POST("")
// }

// SetupRoutesWithDefaults loads config from file and sets up routes with default configuration
func SetupRoutesWithDefaults() *gin.Engine {
	config, err := config.LoadConfig("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return SetupRoutes(config)
}
