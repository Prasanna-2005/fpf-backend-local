package ctrl

import (
	"fmt"
	"net/http"

	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

// Login handles the user login request
func Login(c *gin.Context) {

	var user models.User
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload"})
		return
	}

	// Find user by username and password
	user, err := validateCredentials(payload.Username, payload.Password)
	if err != nil {
		if err.Error() == "record not found" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error1"})
		}
		return
	}
	cfg := utility.LoadOTPConfig()
	code, hash, err := utility.GenerateSecureOTP(cfg.OTPLength)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		return
	}
	err = otpUpdate(hash, &user)
	if err != nil {
		fmt.Print(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.UserName,
			"email":    user.Email,
		},
		"otp_code": code,
	})

}

func validateCredentials(name string, password string) (models.User, error) {
	db := db.GetDB()
	var user models.User
	if err := db.Where("username = ? AND password = ?", name, password).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return models.User{}, fmt.Errorf("record not found")
		}
		return models.User{}, fmt.Errorf("internal server error: %w", err)
	}
	return user, nil
}

func otpUpdate(hash string, user *models.User) error {
	db := db.GetDB()
	otp := models.OTP{
		UserID:  user.ID,
		OTPCode: hash,
	}
	// Insert the OTP into the database
	if err := db.Create(&otp).Error; err != nil {
		return err
	}
	return nil
}
