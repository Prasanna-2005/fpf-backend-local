package db

import (
	"fmt"
	"form_pf_backend/models"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

// var logger = utility.LoggerFunc()
const (
	DbError = "Internal server error: "
	idQuery = "id = ?"
)

func ValidateCredentials(name, password string) (models.User, error) {

	var u models.User
	if !dbAlive(sqlDb) {
		return models.User{}, fmt.Errorf("db connection lost")
	}
	if err := sqlDb.Where("emailAddress = ?", name).First(&u).Error; err != nil {
		return models.User{}, fmt.Errorf("invalid Email Address")
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
	if err := sqlDb.Where(idQuery, userId).First(&user).Error; err != nil {
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

func UpdatePasswd(user *models.User) error {
	return sqlDb.Save(&user).Error
}

func CreateUser(user *models.User) (string, error) {
	if err := sqlDb.Create(&user).Error; err != nil {
		return "", err
	}
	return user.ID, nil
}

func GetUserEmail(email string) (bool, error) {
	var exists bool
	err := sqlDb.Model(&models.User{}).
		Select("count(*) > 0").
		Where("emailAddress = ?", email).
		Find(&exists).Error
	if err != nil {
		return false, err // DB failure
	}
	if exists {
		return true, nil
	}
	return false, nil
}

func GetAllUser() (*[]models.User, error) {
	var users []models.User
	err := sqlDb.Model(&models.User{}).
		Select("id", "userName", "emailAddress", "accessType", "clientId", "createdAt", "updatedAt", "updatedBy").
		Find(&users).Error
	return &users, err
}

func RemoveUser(uid string) error {
	// 1. fetch the user row
	var u models.User
	if err := sqlDb.First(&u, idQuery, uid).Error; err != nil {
		return err
	}

	// 2. update filing_tracker rows: copy createdBy (int) -> userName (varchar)
	if err := sqlDb.Model(&models.FilingTracker{}).
		Where("created_by = ?", uid).
		Update("user_name", u.UserName+"(deleted)").Error; err != nil {
		return err
	}

	// 3. delete the user (which cascades to session_logs & filing_tracker)
	return sqlDb.Delete(&models.User{}, "id = ?", uid).Error
}

func GetClient(cid string) (string, error) {
	if strings.EqualFold(cid, "system") {
		return "", fmt.Errorf("No client found")
	}
	var user models.Client
	// Correct the Where clause and pass the userId as a separate argument
	if err := sqlDb.Where(idQuery, cid).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return "", fmt.Errorf("No client found")
		}
		return "", err
	}
	return user.ClientCode, nil

}
