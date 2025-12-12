package modules

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/utility"
)

var (
	logger = utility.LoggerFunc()
)

func OtpService(userId string) error {
	/*
		OtpService generates and sends OTP to the specified user via email.

		This function handles the complete OTP workflow including TOTP generation,
		user lookup, email creation, and delivery.

		:param userId: Unique identifier of the user requesting OTP
		:type userId: string

		:returns: Error if any step in the OTP process fails, nil on success
		:rtype: error

		Process Flow:
		1. Retrieves OTP configuration
		2. Generates TOTP code for the user
		3. Fetches user details from database
		4. Creates formatted OTP email
		5. Sends email to user's registered address
	*/

	logger.Log("INFO", "Starting OTP service for user")

	// Get OTP configuration
	otpCfg := config.OTP()

	// Generate TOTP code
	logger.Log("INFO", "Generating TOTP code")
	code, err := utility.GenTOTP(&otpCfg, userId)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to generate TOTP for user : %v", err))
		return err
	}
	logger.Log("INFO", "TOTP code generated successfully for user")

	// Get user details
	logger.Log("INFO", "Fetching user details ")
	user, err := db.GetUser(userId)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to retrieve user from database: %v", err))
		return err
	}

	// Create email content
	logger.Log("INFO", "Creating OTP email content")
	emailOTP, err := utility.CreateQuantriumEmail(code, user.UserName, otpCfg.OTPExpiry.String())
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create OTP email for user %s: %v", userId, err))
		return err
	}
	logger.Log("INFO", "OTP email content created successfully")

	// Send OTP email

	err = utility.SendOTPEmail(user.Email, emailOTP)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to send OTP email to %s: %v", user.Email, err))
		return err
	}

	logger.Log("INFO", "OTP service completed successfully for user")
	return nil
}
