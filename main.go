package main

import (
	"log"

	"form_pf_backend/server"
	"form_pf_backend/utility"

	"github.com/go-ini/ini"
	"github.com/joho/godotenv"
)

func main() {
	// Initialize logger
	if err := utility.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	utility.Info("Application starting...")

	// Load environment variables from .env file
	utility.Debug("Loading environment variables from .env file")
	err := godotenv.Load()
	if err != nil {
		utility.Errorf("Error loading .env file: %v", err)
		log.Fatalf("Error loading .env file: %v", err)
	}
	utility.Info("Environment variables loaded successfully")

	// Set up routes
	utility.Debug("Setting up application routes")
	r := server.SetupRoutesWithDefaults()
	utility.Info("Routes configured successfully")

	// Load config to get port
	utility.Debug("Loading configuration for server port")
	cfg, err := ini.Load("config/config.ini")

	if err != nil {
		utility.Errorf("Failed to load config: %v", err)
		log.Fatalf("Failed to load config: %v", err)
	}

	port := cfg.Section("server").Key("port").String()
	utility.Infof("Server will run on port: %s", port)

	// Start the server
	utility.Infof("Starting server on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		utility.Errorf("Failed to start server: %v", err)
		log.Fatalf("Failed to start server: %v", err)
	}
}
