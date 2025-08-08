package middleware

import (
	"errors"
	"fmt"
	"form_pf_backend/utility"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	AuthError   = "User not found / Invalid User session"
	Restriction = "Insufficient privellege.Contact admin"
	UidError    = "User Id not present in Header"
)

var logger = utility.LoggerFunc()

func Authenticate(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid, err := ExtractUserID(c)
		if err != nil {
			logger.Log("WARNING", "Payload doesn't have user id: "+err.Error())
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": UidError})
			return
		}
		var cookieName = uid + "access"
		for _, i := range c.Request.Cookies() {
			if strings.Compare(cookieName, i.Name) == 0 {
				cookieName = i.Name
				break
			}
		}
		if cookieName == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "access cookie not found"})
			return
		}
		token, err := c.Cookie(cookieName)
		if token == "" || err != nil {
			logger.Log("ERROR", "Authenticate error: "+err.Error()+cookieName)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
			return
		}

		claims, err := utility.VerifyAccessToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": AuthError})
			return
		}

		userID := claims.ID
		role := claims.AccessType
		fmt.Println(role)
		if requiredRole != "" && role != requiredRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": Restriction})
			return
		}

		c.Set("userId", userID)
		c.Set("role", role)

		c.Next()
	}
}

// Returns the user_id (string) or an error.
// Safe to call from middleware; downstream handlers can still bind JSON.
func ExtractUserID(c *gin.Context) (string, error) {
	id := c.GetHeader("X-User-ID")
	if id == "" {
		return "", errors.New("user_id missing in header")
	}
	return id, nil
}
