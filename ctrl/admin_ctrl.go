package ctrl

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const UserExist = "User already exists"

type userData struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Password string `json:"password"`
	UserRole string `json:"user_role"`
	Email    string `json:"email"`
}

type NewUserRequest struct {
	ClientCode string   `json:"client_code" binding:"required"`
	Data       userData `json:"data" binding:"required"`
}

type EditUserRequest struct {
	ClientCode string   `json:"client_code"`
	Data       userData `json:"data" binding:"required"`
}

func callerClient(c *gin.Context) (string, bool) {
	id, ok := c.Get("clientId")
	client, _ := id.(string)
	if !ok || client == "" {
		return "", false
	}
	return client, true
}

func NewUser(c *gin.Context) {
	var body NewUserRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		logger.Log("WARNING", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	req := body.Data
	if req.Username == "" || req.Password == "" || req.UserRole == "" || req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	if err := utility.ValidateUser(req.Username, req.Email); err != nil {
		logger.Log("WARNING", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	email, err := db.FindEmail(req.Email)
	if email {
		logger.Log("WARNING", UserExist)
		c.JSON(http.StatusBadRequest, gin.H{"message": UserExist})
		return
	}
	if err != nil {
		logger.Log("ERROR", "DB error: "+err.Error())
	}

	// Hash the password
	cost := config.BcryptCost()
	hashedPasswd, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		cost.Cost,
	)

	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("Password hashing failed: %v", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	adminId, ok := c.Get("userId")
	if !ok || adminId.(string) == "" {
		logger.Log("ERROR", "Missing userId in token claims")
		c.JSON(http.StatusForbidden, gin.H{"message": ServerError})
		return
	}

	jti := int(time.Now().Unix())

	user := models.User{
		UserName:   req.Username,
		Email:      req.Email,
		Password:   string(hashedPasswd),
		AccessType: req.UserRole,
		ClientID:   body.ClientCode,
		Jti:        jti,
		CreatedAt:  time.Now().UTC(),
		UpdatedBy:  adminId.(string),
	}

	uid, err := db.CreateUser(&user)

	if err != nil {
		logger.Log("ERROR", "New user not created: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "User not Created"})
		return
	}

	admin, err := db.GetUserByID(adminId.(string))
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("EditUser - user not found [%s]: %v", adminId.(string), err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}
	user.Password = req.Password // converted for mailing the user
	emailTemplate, err := utility.CreateUserProvisionMail(&user, admin)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("user notification email error: %v", err.Error()))
	}
	if emailTemplate != "" {
		err = utility.SendOTPEmail(user.Email, emailTemplate)
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("User enrollment notification email error: %v", err.Error()))
		}
	}
	c.JSON(http.StatusCreated, gin.H{"id": uid})

}

func GetUsers(c *gin.Context) {
	clientCode, ok := c.Get("clientId")
	if !ok || clientCode.(string) == "" {
		logger.Log("ERROR", "ClientCode not present in token")
		c.JSON(http.StatusUnauthorized, gin.H{
			"message": "Unauthorized - missing token claims",
		})
		return
	}
	usersPtr, err := db.GetClientUser(clientCode.(string))
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("DB error:%v", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": ServerError,
		})
		return
	}

	//  success payload
	type respUser struct {
		UserId    string    `json:"user_id"`
		Username  string    `json:"username"`
		Role      string    `json:"role"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		UpdatedBy string    `json:"updated_by"`
	}
	var list []respUser
	for _, u := range *usersPtr {
		list = append(list, respUser{
			UserId:    u.ID,
			Username:  u.UserName,
			Role:      u.AccessType,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
			UpdatedBy: u.UpdatedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{"users": list})
}

func EditUser(c *gin.Context) {
	var body EditUserRequest
	if err := c.ShouldBindJSON(&body); err != nil || body.Data.UserID == "" {
		logger.Log("ERROR", "EditUser - payload missing user_id")
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	payload := body.Data

	user, err := db.GetUserByID(payload.UserID)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("EditUser - user not found [%s]: %v", payload.UserID, err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}

	if err := applyUserUpdates(user, &payload, body.ClientCode); err != nil {
		logger.Log("ERROR", fmt.Sprintf("EditUser - apply updates err: %v", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to process updates"})
		return
	}

	if err := utility.ValidateUser(user.UserName, user.Email); err != nil {
		logger.Log("WARNING", fmt.Sprintf("EditUser - validation err: %v", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	user.UpdatedAt = time.Now()

	if err := db.UpdateUser(user); err != nil {
		logger.Log("ERROR", fmt.Sprintf("EditUser - update err [%s]: %v", payload.UserID, err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update user"})
		return
	}

	logger.Log("INFO", fmt.Sprintf("EditUser - user [%s] updated successfully", payload.UserID))
	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
}

func applyUserUpdates(user *models.User, payload *userData, clientCode string) error {
	if strings.TrimSpace(payload.Username) != "" {
		user.UserName = strings.TrimSpace(payload.Username)
	}
	if strings.TrimSpace(payload.Email) != "" {
		user.Email = strings.TrimSpace(payload.Email)
	}
	if strings.TrimSpace(payload.UserRole) != "" {
		user.AccessType = strings.TrimSpace(payload.UserRole)
	}
	if strings.TrimSpace(clientCode) != "" {
		user.ClientID = strings.TrimSpace(clientCode)
	}
	if strings.TrimSpace(payload.Password) == "" {
		return nil
	}
	cost := config.BcryptCost()
	hashed, err := bcrypt.GenerateFromPassword([]byte(payload.Password), cost.Cost)
	if err != nil {
		return fmt.Errorf("password hash err: %v", err)
	}
	user.Password = string(hashed)
	return nil
}

// func DeleteUser(c *gin.Context) {
// 	userID := c.Param("user_id") // grab the path variable
// 	if userID == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"message": "user_id missing"})
// 		return
// 	}

// 	if err := db.RemoveUser(userID); err != nil {
// 		logger.Log("ERROR", "DeleteUser: "+err.Error())
// 		c.JSON(http.StatusInternalServerError, gin.H{"message": "could not delete user"})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
// }
