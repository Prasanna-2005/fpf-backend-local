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

func ValidateCredentials(name string, password string) (models.User, error) {
	Connect()
	db := GetDB()
	var user models.User
	if err := db.Where("username = ? AND password = ?", name, password).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return models.User{}, fmt.Errorf("record not found")
		}
		return models.User{}, fmt.Errorf("internal server error: %w", err)
	}
	return user, nil
}
