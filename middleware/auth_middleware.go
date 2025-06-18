package middleware

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

func OtpService(userId string) error {
	var emailOTP string
	otpCfg := utility.LoadOTPConfig()
	code, err := utility.GenerateTOTP(otpCfg, userId)
	if err != nil {
		return err
	}

	user, err := db.GetUser(userId)
	if err != nil {
		return err
	}
	emailOTP, err = utility.CreateQuantriumEmail(code, user.UserName, otpCfg.OTPExpiry.String())
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Printf("email: %s ", user.Email)
	err = utility.SendOTPEmail(user.Email, emailOTP)
	if err != nil {
		return err
	}

	return nil
}

func Authenticate(c *gin.Context) {
	token := c.Request.Header.Get("Authorization")

	if token == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Not authorized."})
		return
	}

	userId, err := utility.VerifyToken(token)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "Not authorized."})
		return
	}

	c.Set("userId", userId)
	c.Next()
}
