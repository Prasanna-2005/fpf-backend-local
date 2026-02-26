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
			AllowHeaders:     cfg.CORSHeaders,
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
	setUpGeneralRoutes(api)
	setUpSubmission(api)
	setupProfileRoutes(api)
	setupAdminRoutes(api)
	// setupFundRoutes(api)
	setupFileRoutes(api)
	setUpEdit(api)
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

func setUpGeneralRoutes(api *gin.RouterGroup) {
	gen := api.Group("")
	gen.Use(middleware.Authenticate())
	gen.GET("/filings_list", ctrl.GetFilings)
	// gen.GET("/response/:filing_id", ctrl.EditDoc)
	gen.GET("/fund_details", ctrl.GetFunds)
	gen.GET("/status", ctrl.FilingStatus)
	gen.GET("/verify_user", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})
}

func setUpSubmission(api *gin.RouterGroup) {
	sub := api.Group("/submissions")
	sub.Use(middleware.Authenticate(), middleware.RequireFilingAccess())
	sub.PUT("/:filing_id/:fund_id", ctrl.AcceptItemValue)
	sub.PUT("revert/:filing_id/:fund_id", ctrl.RevertLatest)
	sub.GET("/history", ctrl.GetFieldHistory)
	// sub.PUT("/:filing_id/approval", ctrl.SendForApproval)
	// sub.PUT("/:filing_id/approved", ctrl.MarkApproved)
}

func setupProfileRoutes(api *gin.RouterGroup) {
	prof := api.Group("/profile")
	prof.Use(middleware.Authenticate())
	prof.POST("/reset_password", ctrl.UpdatePasswd)

}

func setupAdminRoutes(api *gin.RouterGroup) {
	admin := api.Group("/admin")
	admin.Use(middleware.Authenticate("admin"))
	admin.GET("/users", ctrl.GetUsers)
	admin.POST("/create_user", ctrl.NewUser)
	// admin.DELETE("/delete_user/:user_id", ctrl.DeleteUser)

}

func setupFileRoutes(api *gin.RouterGroup) {
	file := api.Group("/files")
	file.Use(middleware.Authenticate("contributor", "approver", "admin"))
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	file.POST("/upload", ctrl.UploadDoc(cfg.UploadVolumePath))
	file.POST("/mapper", ctrl.MapFiles)
	file.POST("/submit", ctrl.SubmitFiles)
	file.POST("/cancel", ctrl.CancelFiling(cfg.UploadVolumePath))
	file.GET("/download_input_files", ctrl.DownloadFiles(cfg.UploadVolumePath))

}

func setUpEdit(api *gin.RouterGroup) {
	edit := api.Group("/edit")
	edit.Use(middleware.Authenticate("contributor", "approver", "admin"), middleware.RequireFilingAccess())
	edit.GET("/response/:filing_id", ctrl.GetFilingResponse)
	edit.GET("/approved/:filing_id", ctrl.GetApprovedFilingResponse)
	edit.POST("/update_status/:filing_id", ctrl.UpdateRequestTrackerStatus)
	edit.GET("/computation_logs/:filing_id", ctrl.GetCompLogs)
}

// SetupRoutesWithDefaults loads config from file and sets up routes with default configuration
func SetupRoutesWithDefaults() *gin.Engine {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return SetupRoutes(config)
}
