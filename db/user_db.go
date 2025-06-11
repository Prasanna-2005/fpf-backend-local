package db

import (
	"fmt"
	"form_pf_backend/models"

	"github.com/jinzhu/gorm"
)

func OtpUpdate(hash string, user *models.User) error {
	Connect()
	database := GetDB()
	otp := models.OTP{
		UserID:  user.ID,
		OTPCode: hash,
	}
	// Insert the OTP into the database
	if err := database.Create(&otp).Error; err != nil {
		return err
	}
	return nil
}
