package db

import (
	"fmt"
	"form_pf_backend/models"
	"time"

	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

// var logger = utility.LoggerFunc()
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
		UserID:    user.ID,
		ClientID:  user.ClientID,
		LoginTime: time.Now().UTC(),
		Jti:       user.Jti,
	}

	return sqlDb.Create(&session).Error

}

// func UpdateLogout(uid, expiry string) error {
// 	// 1. Parse the incoming string into UTC time
// 	logoutTime, err := time.Parse(time.RFC3339, expiry)
// 	if err != nil {
// 		return err
// 	}
// 	logoutTime = logoutTime.UTC()

// 	// 2. Get the user so we know the active JWT id
// 	user, err := GetUser(uid)
// 	if err != nil {
// 		return err
// 	}

// 	// 3. Update the session row
// 	return sqlDb.Model(&models.Session{}).
// 		Where("userId = ? AND jwtId = ?", uid, user.Jti).
// 		Update("logoutTime", logoutTime).Error
// }

func UpdateLogout(uid, expiry string) error {
	// 1. Parse the incoming string into UTC time
	logoutTime, err := time.Parse(time.RFC3339, expiry)
	if err != nil {
		// logger.Log("ERROR", "logout parsing error: "+err.Error())
		fmt.Print("parse: " + err.Error())
		return err
	}
	logoutTime = logoutTime.UTC()

	// 2. Get the user so we know the active JWT id
	user, err := GetUser(uid)
	if err != nil {
		// logger.Log("ERROR", "User not found: "+err.Error())
		fmt.Println("uid error: " + err.Error())
		return err
	}

	// 3. Update the session row
	if err := sqlDb.Model(&models.Session{}).
		Where("userId = ? AND jwtId = ?", uid, user.Jti).
		Update("logoutTime", logoutTime).Error; err != nil {
		return err
	}

	// 4. Increment JTI in users table
	user.Jti += 1
	return sqlDb.Save(&user).Error
	// return sqlDb.Model(&models.User{}).
	// 	Where("id = ?", uid).
	// 	Update("jwtId").Error
}
