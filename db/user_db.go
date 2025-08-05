package db

import (
	"fmt"
	"form_pf_backend/models"
	"time"

	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

var DbError = "Internal server error: "

func ValidateCredentials(name, password string) (models.User, error) {
	var u models.User
	if err := sqlDb.Where("username = ?", name).First(&u).Error; err != nil {
		return models.User{}, fmt.Errorf("invalid username ")
	}

	match := func(h, p string) bool {
		return bcrypt.CompareHashAndPassword([]byte(h), []byte(p)) == nil
	}
	if !match(u.Password, password) {
		return models.User{}, fmt.Errorf("invalid  password")
	}
	return u, nil
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

func UpdateLogout(uid, expiry string) error {
	// 1. Parse the incoming string into UTC time
	logoutTime, err := time.Parse(time.RFC3339, expiry)
	if err != nil {
		return err
	}
	logoutTime = logoutTime.UTC()

	// 2. Get the user so we know the active JWT id
	user, err := GetUser(uid)
	if err != nil {
		return err
	}

	// 3. Update the session row in one GORM call
	return sqlDb.Model(&models.Session{}).
		Where("userId = ? AND jwtId = ?", uid, user.Jti).
		Update("logoutTime", logoutTime).Error
}
