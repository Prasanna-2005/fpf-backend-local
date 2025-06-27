package middleware

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/utility"
)

var (
	// Global logger instance
	logger = utility.LoggerFunc()
)

func OtpService(userId string) error {
	logger.Log("INFO", fmt.Sprintf("Starting OTP service for user: %s", userId))

	// Get OTP configuration
	otpCfg := config.OTP()
	logger.Log("DEBUG", "Retrieved OTP configuration")

	// Generate TOTP code
	logger.Log("DEBUG", "Generating TOTP code")
	code, err := utility.GenTOTP(&otpCfg, userId)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to generate TOTP for user %s: %v", userId, err))
		return err
	}
	logger.Log("INFO", fmt.Sprintf("TOTP code generated successfully for user: %s", userId))

	// Get user details
	logger.Log("DEBUG", fmt.Sprintf("Fetching user details for ID: %s", userId))
	user, err := db.GetUser(userId)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to retrieve user %s from database: %v", userId, err))
		return err
	}
	logger.Log("INFO", fmt.Sprintf("User details retrieved: %s (%s)", user.UserName, user.Email))

	// Create email content
	logger.Log("DEBUG", "Creating OTP email content")
	emailOTP, err := utility.CreateQuantriumEmail(code, user.UserName, otpCfg.OTPExpiry.String())
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create OTP email for user %s: %v", userId, err))
		return err
	}
	logger.Log("DEBUG", "OTP email content created successfully")

	// Send OTP email
	logger.Log("INFO", fmt.Sprintf("Sending OTP email to: %s", user.Email))
	err = utility.SendOTPEmail(user.Email, emailOTP)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to send OTP email to %s: %v", user.Email, err))
		return err
	}

	logger.Log("INFO", fmt.Sprintf("OTP service completed successfully for user: %s", userId))
	return nil
}
