package modules

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/models"
	"form_pf_backend/utility"
	"time"
)

var (
	logger = utility.LoggerFunc()
)

func OtpService(user *models.User) error {
	/*
		OtpService generates an OTP and starts the email send without waiting for Gmail.
		Login uses this so the password check can return immediately.
	*/
	emailOTP, err := buildOTPEmail(user)
	if err != nil {
		return err
	}

	to := user.Email
	userID := user.ID
	go func() {
		if err := utility.SendOTPEmail(to, emailOTP); err != nil {
			logger.Log("ERROR", fmt.Sprintf("Failed to send OTP email to %s: %v", to, err))
			return
		}
		logger.Log("INFO", "OTP email sent for user "+userID)
	}()
	return nil
}

func SendOtpAndWait(user *models.User) error {
	/*
		SendOtpAndWait generates an OTP and returns only after Gmail accepts or rejects it.
		Resend uses this so a failed delivery is reported to the user.
	*/
	emailOTP, err := buildOTPEmail(user)
	if err != nil {
		return err
	}
	if err := utility.SendOTPEmail(user.Email, emailOTP); err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to send OTP email to %s: %v", user.Email, err))
		return err
	}
	logger.Log("INFO", "OTP email sent for user "+user.ID)
	return nil
}

func buildOTPEmail(user *models.User) (string, error) {
	if user == nil || user.ID == "" || user.Email == "" {
		return "", fmt.Errorf("missing user for OTP")
	}

	logger.Log("INFO", "Starting OTP service for user")
	otpCfg := config.OTP()

	logger.Log("INFO", "Generating TOTP code")
	code, err := utility.GenTOTP(&otpCfg, user.ID)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to generate TOTP for user : %v", err))
		return "", err
	}
	logger.Log("INFO", "TOTP code generated successfully for user")

	logger.Log("INFO", "Creating OTP email content")
	expiry := otpCfg.OTPExpiry.String()
	if otpCfg.OTPExpiry == time.Minute {
		expiry = "1 minute"
	}
	emailOTP, err := utility.CreateQuantriumEmail(code, user.UserName, expiry)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Failed to create OTP email for user %s: %v", user.ID, err))
		return "", err
	}
	logger.Log("INFO", "OTP email content created successfully")
	return emailOTP, nil
}
