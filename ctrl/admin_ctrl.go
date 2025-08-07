package ctrl

import (
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const UserExist = "User already exists"

type NewUserRequest struct {
	Data struct {
		Username  string `json:"username" binding:"required"`
		Password  string `json:"password" binding:"required"`
		UserRole  string `json:"user_role" binding:"required"`
		CreatedAt string `json:"created_at"`
		Email     string
	} `json:"data"`
	Role       string `json:"role"`
	AdminID    string `json:"admin_id"`
	ClientCode string `json:"client_code"`
}

func NewUser(c *gin.Context) {
	var req NewUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	email, err := db.GetUserEmail(req.Data.Email)
	if err != nil || !email {
		logger.Log("WARNING", "User already exist")
		c.JSON(http.StatusBadRequest, gin.H{"message": UserExist})
	}

	// Hash the password
	cost := config.BcryptCost()
	hashedPasswd, err := bcrypt.GenerateFromPassword(
		[]byte(req.Data.Password),
		cost.Cost,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	user := models.User{
		UserName:   req.Data.Username,
		Password:   string(hashedPasswd),
		Email:      req.Data.Email,
		AccessType: req.Role,
		CreatedAt:  time.Now().UTC(),
		UpdatedBy:  "admin",
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
	usersPtr, err := db.GetAllUser()
	if err != nil {
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

func DeleteUser(c *gin.Context) {

}
