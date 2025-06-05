package ctrl

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
)

// Login handles the user login request
func Login(c *gin.Context) {
	var payload struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload"})
		return
	}

	var user models.User
	db := db.GetDB()

	// Find user by username and password
	if err := db.Where("username = ? AND password = ?", payload.Username, payload.Password).First(&user).Error; err != nil {
		log.Printf("Database query failed: %v", err) // Log the error

		if gorm.IsRecordNotFoundError(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Incorrect Username or Password"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		}
		return
	}

	// Generate OTP
	cfg := utility.LoadOTPConfig()
	otp, otpHash, err := utility.GenerateSecureOTP(cfg.OTPLength)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		return
	}

	// Save OTP hash and expiry in the database
	user.OTPHash = otpHash
	user.OTPExpiry = time.Now().Add(cfg.OTPExpiry)

	// // Save user with OTP details
	// if err := db.Save(&user).Error; err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
	// 	return
	// }

	// Send OTP to user
	if err := utility.SendOTPEmail(cfg, "anandhakumar.u@quantrium.ai", otp); err != nil {
		fmt.Print(user.EmailAddress)
		log.Printf("Failed to send OTP: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP sent successfully",
		"user_id": user.ID,
	})
}

// VerifyOTP handles the OTP verification request
func VerifyOTP(c *gin.Context) {
	var payload struct {
		UserID string `json:"user_id" binding:"required"`
		OTP    string `json:"otp" binding:"required"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload"})
		return
	}

	var user models.User
	db := db.GetDB()

	// Find user by user_id
	if err := db.Where("id = ?", payload.UserID).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired OTP"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error. Please reach out to your admin"})
		}
		return
	}

	hashOtp, err := utility.HashOTP(payload.OTP)
	if err != nil {
		return
	}
	// Verify OTP
	if user.OTPExpiry.Before(time.Now()) || user.OTPHash != hashOtp {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Invalid or expired OTP"})
		return
	}

	// Clear OTP after verification
	user.OTPHash = ""
	user.OTPExpiry = time.Time{}

	// Save user
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error. Please reach out to your admin"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verified successfully",
	})
}
