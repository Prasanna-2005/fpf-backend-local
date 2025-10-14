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
	Restriction = "Insufficient privilege. Contact admin"
	UidError    = "User Id not present in Header"
)

var logger = utility.LoggerFunc()

/*
	------------------------------------------------------------------
	  Public API – three ways to call:
	    auth.Authenticate("admin")
	    auth.Authenticate([]string{"admin","user"})
	    auth.Authenticate("admin","user")

------------------------------------------------------------------
*/

func Authenticate(required ...interface{}) gin.HandlerFunc {
	allowed := make([]string, 0, len(required))
	for _, r := range required {
		switch v := r.(type) {
		case string:
			allowed = append(allowed, v)
		case []string:
			allowed = append(allowed, v...)
		default:
			panic(fmt.Sprintf("Authenticate: unsupported type %T", v))
		}
	}

	return func(c *gin.Context) {
		uid, err := ExtractUserID(c)
		if err != nil {
			abortWithMsg(c, http.StatusBadRequest, UidError,
				"Header doesn't have user id: "+err.Error())
			return
		}

		token, ok := findAccessToken(c, uid)
		if !ok {
			return
		}

		claims, err := utility.VerifyAccessToken(token)
		if err != nil {
			if strings.Contains(err.Error(), "expired") {
				abortWithMsg(c, http.StatusUnauthorized,
					"Token expired. Please login again",
					"Token expired for user: "+uid)
				return
			}
			if strings.Contains(err.Error(), "revoked") {
				abortWithMsg(c, http.StatusUnauthorized,
					"Session terminated. Please login again",
					"Revoked token for user: "+uid)
				return
			}
			abortWithMsg(c, http.StatusUnauthorized, AuthError,
				"Token verification failed for user "+uid+": "+err.Error())
			return
		}

		if !roleAllowed(claims.AccessType, allowed) {
			abortWithMsg(c, http.StatusForbidden, Restriction,
				fmt.Sprintf("User %s with role '%s' attempted to access restricted resource",
					claims.ID, claims.AccessType))
			return
		}

		logger.Log("DEBUG", fmt.Sprintf("Authenticated user: %s, role: %s, clientID: %v",
			claims.ID, claims.AccessType, claims.ClientID))

		c.Set("userId", claims.ID)
		c.Set("role", claims.AccessType)
		c.Set("clientId", claims.ClientID)
		c.Next()
	}
}

func abortWithMsg(c *gin.Context, code int, msg, logMsg string) {
	if logMsg != "" {
		logger.Log("WARNING", logMsg)
	}
	c.AbortWithStatusJSON(code, gin.H{"message": msg})
}

func findAccessToken(c *gin.Context, uid string) (string, bool) {
	cookieName := uid + "access"
	if _, err := c.Cookie(cookieName); err != nil {
		logger.Log("DEBUG", "Cookie not found: "+cookieName)
		abortWithMsg(c, http.StatusUnauthorized,
			"Access token not found",
			"Missing cookie '"+cookieName+"': "+err.Error())
		return "", false
	}

	token, _ := c.Cookie(cookieName)
	if token == "" {
		abortWithMsg(c, http.StatusUnauthorized, AuthError,
			"Empty access token in cookie: "+cookieName)
		return "", false
	}

	return token, true
}

func roleAllowed(userRole string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, r := range allowed {
		if r == userRole {
			return true
		}
	}
	return false
}

func ExtractUserID(c *gin.Context) (string, error) {
	id := c.GetHeader("X-User-ID")
	if id == "" {
		return "", errors.New("user_id missing in header")
	}
	return id, nil
}
