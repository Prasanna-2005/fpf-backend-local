package middleware

import (
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/utility"
)

func OtpService(userId string) error {
	utility.Infof("Starting OTP service for user: %s", userId)

	// Get OTP configuration
	otpCfg := config.OTP()
	utility.Debug("Retrieved OTP configuration")

	// Generate TOTP code
	utility.Debug("Generating TOTP code")
	code, err := utility.GenTOTP(&otpCfg, userId)
	if err != nil {
		utility.Errorf("Failed to generate TOTP for user %s: %v", userId, err)
		return err
	}
	utility.Infof("TOTP code generated successfully for user: %s", userId)

	// Get user details
	utility.Debugf("Fetching user details for ID: %s", userId)
	user, err := db.GetUser(userId)
	if err != nil {
		utility.Errorf("Failed to retrieve user %s from database: %v", userId, err)
		return err
	}
	utility.Infof("User details retrieved: %s (%s)", user.UserName, user.Email)

	// Create email content
	utility.Debug("Creating OTP email content")
	emailOTP, err := utility.CreateQuantriumEmail(code, user.UserName, otpCfg.OTPExpiry.String())
	if err != nil {
		utility.Errorf("Failed to create OTP email for user %s: %v", userId, err)
		return err
	}
	utility.Debug("OTP email content created successfully")

	// Send OTP email
	utility.Infof("Sending OTP email to: %s", user.Email)
	err = utility.SendOTPEmail(user.Email, emailOTP)
	if err != nil {
		utility.Errorf("Failed to send OTP email to %s: %v", user.Email, err)
		return err
	}

	utility.Infof("OTP service completed successfully for user: %s", userId)
	return nil
}
