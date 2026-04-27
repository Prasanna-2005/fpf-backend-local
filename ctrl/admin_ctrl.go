package ctrl

import (
	"fmt"
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const UserExist = "User already exists"

type NewUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	UserRole string `json:"user_role" binding:"required"`
	Email    string `json:"email" binding:"required"`
}

func NewUser(c *gin.Context) {
	var req NewUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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
		logger.Log("ERROR", fmt.Sprintf("Missing userId in token claims: %v", err.Error()))
		c.JSON(http.StatusForbidden, gin.H{"message": ServerError})
	}

	cid, ok := c.Get("clientId")
	if !ok || cid.(string) == "" {
		logger.Log("ERROR", fmt.Sprintf("Client id parsing error: %v", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	jti := int(time.Now().Unix())

	user := models.User{
		UserName:   req.Username,
		Email:      req.Email,
		Password:   string(hashedPasswd),
		AccessType: req.UserRole,
		ClientID:   cid.(string),
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
	var payload struct {
		UserId   string  `json:"user_id" binding:"required"`
		Username *string `json:"username"`
		Password *string `json:"password"`
		UserRole *string `json:"user_role"`
		Email    *string `json:"email"`
	}

	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", fmt.Sprintf("payload err:%v", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	// Fetch existing user
	user, err := db.GetUserByID(payload.UserId)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("user not found: %v", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
		return
	}

	// Apply only provided fields (partial update)
	if payload.Username != nil {
		user.Username = *payload.Username
	}
	if payload.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*payload.Password), bcrypt.DefaultCost)
		if err != nil {
			logger.Log("ERROR", fmt.Sprintf("password hash err: %v", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to process password"})
			return
		}
		user.Password = string(hashedPassword)
	}
	if payload.UserRole != nil {
		user.UserRole = *payload.UserRole
	}
	if payload.Email != nil {
		user.Email = *payload.Email
	}

	// Save updated user
	if err := db.UpdateUser(user); err != nil {
		logger.Log("ERROR", fmt.Sprintf("update user err: %v", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update user"})
		return
	}

	logger.Log("INFO", fmt.Sprintf("user %s updated successfully", payload.UserId))
	c.JSON(http.StatusOK, gin.H{"message": "User updated successfully"})
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
