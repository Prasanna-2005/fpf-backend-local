// Package server provides HTTP server setup and route configuration
package server

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/ctrl"
	"form_pf_backend/middleware"
	"net/http"
	"strings"

	"log"

	"github.com/gin-gonic/gin"
)

// Manual CORS middleware
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		fmt.Printf("🌐 Incoming request - Method: %s, Origin: %s, Path: %s\n",
			c.Request.Method, origin, c.Request.URL.Path)

		// Allow specific origins - ADD YOUR FRONTEND URLs HERE
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:3001",
			"http://localhost:5173", // Vite default
			"http://localhost:3002",
		}

		// Check if origin is allowed
		originAllowed := false
		for _, allowed := range allowedOrigins {
			if origin == allowed {
				originAllowed = true
				break
			}
		}

		// Set CORS headers
		if originAllowed {
			c.Header("Access-Control-Allow-Origin", origin)
			fmt.Printf("✅ Origin allowed: %s\n", origin)
		} else if origin != "" {
			fmt.Printf("❌ Origin not allowed: %s\n", origin)
		}

		// Always set these headers
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Accept,Content-Type,X-User-ID,Authorization,Origin,X-Requested-With,Access-Control-Request-Method,Access-Control-Request-Headers")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		// Print all CORS headers being sent
		fmt.Printf("📋 CORS Headers being sent:\n")
		fmt.Printf("  Access-Control-Allow-Origin: %s\n", c.Writer.Header().Get("Access-Control-Allow-Origin"))
		fmt.Printf("  Access-Control-Allow-Methods: %s\n", c.Writer.Header().Get("Access-Control-Allow-Methods"))
		fmt.Printf("  Access-Control-Allow-Headers: %s\n", c.Writer.Header().Get("Access-Control-Allow-Headers"))
		fmt.Printf("  Access-Control-Allow-Credentials: %s\n", c.Writer.Header().Get("Access-Control-Allow-Credentials"))

		// Handle preflight requests (OPTIONS)
		if c.Request.Method == "OPTIONS" {
			fmt.Printf("🚁 Handling preflight OPTIONS request\n")

			// Get requested method and headers from preflight
			requestMethod := c.Request.Header.Get("Access-Control-Request-Method")
			requestHeaders := c.Request.Header.Get("Access-Control-Request-Headers")

			fmt.Printf("  Requested Method: %s\n", requestMethod)
			fmt.Printf("  Requested Headers: %s\n", requestHeaders)

			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()

		fmt.Printf("✅ Request completed - Status: %d\n", c.Writer.Status())
		fmt.Println(strings.Repeat("-", 50))
	}
}

// SetupRoutes configures and returns a Gin engine with routes and middleware
func SetupRoutes(cfg *config.RouteConfig) *gin.Engine {
	r := gin.Default()

	// Use manual CORS middleware
	r.Use(CORSMiddleware())

	fmt.Println("🔧 Manual CORS middleware enabled")

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
	gen.Use(middleware.Authenticate(""))
	gen.GET("/filings_list", ctrl.GetFilings)
	// gen.GET("/response/:filing_id", ctrl.EditDoc)
	gen.GET("/fund_details", ctrl.GetFunds)
	gen.GET("/status", ctrl.FilingStatus)
}

func setUpSubmission(api *gin.RouterGroup) {
	sub := api.Group("/submissions")
	sub.Use(middleware.Authenticate(""))
	sub.PUT("/:filing_id", ctrl.AcceptItemValue)
	// 	sub.GET("/:filing_id/revert", ctrl.RevertLatest)
	// 	sub.POST("/:filing_id/draft", ctrl.SaveDraft)
	// sub.PUT("/:filing_id/approval", ctrl.SendForApproval)
	sub.PUT("/:filing_id/approved", ctrl.MarkApproved)
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
	file.Use(middleware.Authenticate("contributor"))
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	file.POST("/upload", ctrl.UploadDoc(cfg.UploadVolumePath))
	// 	file.POST("/mapper",ctrl.MapFiles)
	// 	file.POST("/submit",ctrl.SubmitFiles)
}

// SetupRoutesWithDefaults loads config from file and sets up routes with default configuration
func SetupRoutesWithDefaults() *gin.Engine {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	return SetupRoutes(config)
}
