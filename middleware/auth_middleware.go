package middleware

import (
	"form_pf_backend/utility"
	"net/http"

	"github.com/gin-gonic/gin"
)

var AuthError = "User not found / Invalid User session"

func Authenticate(context *gin.Context) {
	token := c.Cookie("access")

	if token == "" {
		context.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
		return
	}

	userId, err := utility.VerifyAccessToken(token)

	if err != nil {
		context.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
		return
	}

	context.Set("userId", userId)
	context.Next()
}
