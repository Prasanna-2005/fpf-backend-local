package ctrl

import (
	"form_pf_backend/config"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/utility"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const UserExist = "User already exists"

type NewUserRequest struct {
	Data struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
		UserRole string `json:"user_role" binding:"required"`
		Email    string
	} `json:"data"`
	ClientCode string `json:"client_code"`
}

func NewUser(c *gin.Context) {
	var req NewUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	if err := utility.ValidateUser(req.Data.Username, req.Data.Email); err != nil {
		logger.Log("WARNING", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	email, err := db.GetUserEmail(req.Data.Email)
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
		[]byte(req.Data.Password),
		cost.Cost,
	)

	if err != nil {
		logger.Log("ERROR", "Password hashing failed: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	role, _ := c.Get("role")

	cid, err := strconv.Atoi(req.ClientCode)
	if err != nil {
		logger.Log("ERROR", "Client id parsing error: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	jti := int(time.Now().Unix())

	user := models.User{
		UserName:   req.Data.Username,
		Password:   string(hashedPasswd),
		Email:      req.Data.Email,
		AccessType: req.Data.UserRole,
		CreatedAt:  time.Now().UTC(),
		UpdatedBy:  role.(string),
		ClientID:   cid,
		Jti:        jti,
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
