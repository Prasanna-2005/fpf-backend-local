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
	fmt.Print(hash)
	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent successfully",
		"user": gin.H{
			"id":       user.ID,
			"username": user.UserName,
			"email":    user.Email, // Assuming you have an Email field in your User model
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

// VerifyOTP handles the OTP verification request
// func VerifyOTP(c *gin.Context) {
// 	var payload struct {
// 		UserID string `json:"user_id" binding:"required"`
// 		OTP    string `json:"otp" binding:"required"`
// 	}

// 	if err := c.ShouldBindJSON(&payload); err != nil {
// 		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload"})
// 		return
// 	}

// 	var user models.User
// 	db := db.GetDB()

// 	// Find user by user_id
// 	if err := db.Where("id = ?", payload.UserID).First(&user).Error; err != nil {
// 		if gorm.IsRecordNotFoundError(err) {
// 			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired OTP"})
// 		} else {
// 			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error. Please reach out to your admin"})
// 		}
// 		return
// 	}

// 	hashOtp, err := utility.HashOTP(payload.OTP)
// 	if err != nil {
// 		return
// 	}
// 	// Verify OTP
// 	if user.OTPExpiry.Before(time.Now()) || user.OTPHash != hashOtp {
// 		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired OTP"})
// 		return
// 	}

// 	// Clear OTP after verification
// 	user.OTPHash = ""
// 	user.OTPExpiry = time.Time{}

// 	// Save user
// 	if err := db.Save(&user).Error; err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error. Please reach out to your admin"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"message": "OTP verified successfully",
// 	})
// }
