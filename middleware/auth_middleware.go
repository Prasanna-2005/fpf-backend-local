package middleware

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/utility"
)

func OtpService(userId string) error {
	var emailOTP string
	cfg := utility.LoadOTPConfig()
	code, err := utility.GenerateTOTP(cfg.SecretKey)
	if err != nil {
		// c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		return err
	}
	// err = db.OtpUpdate(hash, user)
	// if err != nil {
	// 	return "", err
	// }
	user, err := db.GetUser(userId)
	if err != nil {
		return err
	}
	emailOTP, err = utility.CreateQuantriumEmail(code, user.UserName, cfg.OTPExpiry)
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("email: %s ", user.Email)
	err = utility.SendOTPEmail(cfg, user.Email, emailOTP)
	if err != nil {
		// c.JSON(http.StatusFailedDependency, gin.H{"message": "SMTP client or User email doesn't exist"})
		return err
	}

	return nil
}
