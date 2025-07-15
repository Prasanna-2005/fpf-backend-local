package db

import (
	"fmt"
	"form_pf_backend/models"
	"time"

	"github.com/jinzhu/gorm"
)

var DbError = "Internal server error: "

func ValidateCredentials(name string, password string) (models.User, error) {

	var user models.User
	password = Bcrypt()
	if err := sqlDb.Where(" password = ? AND username = ? ", password, name).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return models.User{}, fmt.Errorf("record not found")
		}
		return models.User{}, err
	}
	return user, nil
}

func GetUser(userId string) (*models.User, error) {

	var user models.User
	// Correct the Where clause and pass the userId as a separate argument
	if err := sqlDb.Where("id = ?", userId).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, fmt.Errorf("record not found")
		}
		return nil, err
	}
	return &user, nil
}

func UpdateSession(user *models.User) error {

	var session = models.Session{
		UserID:     user.ID,
		ClientCode: user.ClientCode,
		LoginTime:  time.Now(),
		// ID is omitted - GORM will auto-generate it
	}

	return sqlDb.Create(&session).Error

}

func UpdateToken(user *models.User, access, refresh string) error {

	var token = models.Token{
		UserID:       user.ID,
		ClientCode:   user.ClientCode,
		AccessToken:  access,
		RefreshToken: refresh,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return sqlDb.Create(&token).Error
}
