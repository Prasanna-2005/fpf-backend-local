package db

import (
	"fmt"
	"form_pf_backend/models"

	"github.com/jinzhu/gorm"
)

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

func GetUser(userId string) (*models.User, error) {
	Connect()
	db := GetDB()
	var user models.User
	// Correct the Where clause and pass the userId as a separate argument
	if err := db.Where("id = ?", userId).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, fmt.Errorf("record not found")
		}
		return nil, fmt.Errorf("internal server error: %w", err)
	}
	return &user, nil
}
