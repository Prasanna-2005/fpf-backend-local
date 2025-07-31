package middleware

import (
	"form_pf_backend/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

var AuthError = "User not found / Invalid User session"
var logger = utility.LoggerFunc()

func Authenticate(c *gin.Context) {
	token, err := c.Cookie("access")

	if token == "" || err != nil {
		logger.Log("Error", "Authenticate error: "+err.Error())
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
		return
	}

	userId, err := utility.VerifyAccessToken(token)

	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
		return
	}

	c.Set("userId", userId)
	c.Next()
}
