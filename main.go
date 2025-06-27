package main

import (
	"fmt"
	"log"

	"form_pf_backend/server"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
	"github.com/go-ini/ini"
	"github.com/joho/godotenv"
)

var (
	logger = utility.LoggerFunc()
)

/*
Main function for loading environment variable and starting the server
*/
func main() {
	gin.SetMode(gin.ReleaseMode)
	logger.Log("INFO", "Application starting...")

	// Load environment variables from .env file
	logger.Log("DEBUG", "Loading environment variables from .env file")
	err := godotenv.Load()
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Error loading .env file: %v", err))
		log.Fatalf("Error loading .env file: %v", err)
	}
	logger.Log("INFO", "Environment variables loaded successfully")

	// Set up routes
	logger.Log("DEBUG", "Setting up application routes")
	r := server.SetupRoutesWithDefaults()
	logger.Log("INFO", "Routes configured successfully")

	// Load config to get port
	logger.Log("DEBUG", "Loading configuration for server port")
	cfg, err := ini.Load("config/config.ini")

	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to load config: %v", err))
		log.Fatalf("Failed to load config: %v", err)
	}

	port := cfg.Section("server").Key("port").String()
	logger.Log("INFO", fmt.Sprintf("Server will run on port: %s", port))

	// Start the server
	logger.Log("INFO", fmt.Sprintf("Starting server on port %s...", port))
	if err := r.Run(":" + port); err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to start server: %v", err))
		log.Fatalf("Failed to start server: %v", err)
	}
}
