package server

import (
	"form_pf_backend/config"
	"form_pf_backend/ctrl"

	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(cfg *config.RouteConfig) *gin.Engine {
	r := gin.Default()

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

func setupAPIRoutes(r *gin.Engine, version string) {
	api := r.Group("/api/" + version)
	setupAuthRoutes(api)
}

func setupAuthRoutes(api *gin.RouterGroup) {
	auth := api.Group("/auth")
	auth.POST("/login", ctrl.Login)
	auth.POST("/verify_otp", ctrl.VerifyOTP)
	auth.POST("/resend_otp", ctrl.ResendOTP)

}

func SetupRoutesWithDefaults() *gin.Engine {
	config, err := config.LoadConfig("config.ini")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return SetupRoutes(config)
}
