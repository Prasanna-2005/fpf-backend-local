package db

import (
	"fmt"
	"form_pf_backend/models"
	"time"

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

func UpdateSession(user *models.User) error {
	Connect()
	db := GetDB()
	var session models.Session
	session = models.Session{
		UserID:     user.ID,
		ClientCode: user.ClientCode,
		LoginTime:  time.Now(),
		// ID is omitted - GORM will auto-generate it
	}

	return db.Create(&session).Error

}

func UpdateToken(user *models.User, access, refresh string) error {
	Connect()
	db := GetDB()
	var token models.Token
	token = models.Token{
		UserID:       user.ID,
		ClientCode:   user.ClientCode,
		AccessToken:  access,
		RefreshToken: refresh,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	return db.Create(&token).Error
}
