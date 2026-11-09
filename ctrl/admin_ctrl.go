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

const (
	roleAdmin      = "admin"
	roleSuperAdmin = "superadmin"
)

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

	callerRole, _ := c.Get("role")
	roleName, _ := callerRole.(string)
	if strings.EqualFold(user.AccessType, roleSuperAdmin) && !strings.EqualFold(roleName, roleSuperAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"message": "A client admin cannot change a super admin"})
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

type createClientRequest struct {
	ClientCode    string `json:"client_code" binding:"required"`
	AdminUsername string `json:"admin_username" binding:"required"`
	AdminEmail    string `json:"admin_email" binding:"required"`
	AdminPassword string `json:"admin_password" binding:"required"`
}

func CreateClient(c *gin.Context) {
	var req createClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	req.ClientCode = strings.ToUpper(strings.TrimSpace(req.ClientCode))
	req.AdminUsername = strings.TrimSpace(req.AdminUsername)
	req.AdminEmail = strings.TrimSpace(req.AdminEmail)
	req.AdminPassword = strings.TrimSpace(req.AdminPassword)
	if req.AdminPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	if err := utility.ValidateUser(req.AdminUsername, req.AdminEmail); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	exists, err := db.ClientCodeExists(req.ClientCode)
	if err != nil {
		logger.Log("ERROR", "CreateClient lookup: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	if exists {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Client already exists"})
		return
	}
	emailTaken, err := db.FindEmail(req.AdminEmail)
	if err != nil {
		logger.Log("ERROR", "CreateClient email lookup: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	if emailTaken {
		c.JSON(http.StatusBadRequest, gin.H{"message": UserExist})
		return
	}

	adminID, _ := c.Get("userId")
	actor, _ := adminID.(string)
	now := time.Now().UTC()
	client := models.Client{
		ClientCode: req.ClientCode,
		CreatedAt:  now,
	}
	if err := db.CreateClient(&client); err != nil {
		logger.Log("ERROR", "CreateClient: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Client not created"})
		return
	}

	cost := config.BcryptCost()
	hashed, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), cost.Cost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	admin := models.User{
		UserName:   req.AdminUsername,
		Email:      req.AdminEmail,
		Password:   string(hashed),
		AccessType: roleAdmin,
		ClientID:   req.ClientCode,
		Jti:        int(now.Unix()),
		CreatedAt:  now,
		UpdatedAt:  now,
		UpdatedBy:  actor,
	}
	uid, err := db.CreateUser(&admin)
	if err != nil {
		logger.Log("ERROR", "CreateClient admin: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Client admin not created"})
		return
	}
	admin.ID = uid
	admin.Password = req.AdminPassword
	creator, err := db.GetUserByID(actor)
	if err != nil {
		logger.Log("ERROR", "CreateClient provision mail: "+err.Error())
		c.JSON(http.StatusCreated, gin.H{"id": uid, "client_code": req.ClientCode, "message": "Client created. The account email could not be sent."})
		return
	}
	emailTemplate, err := utility.CreateUserProvisionMail(&admin, creator)
	if err != nil || emailTemplate == "" {
		logger.Log("ERROR", "CreateClient provision mail was not built")
		c.JSON(http.StatusCreated, gin.H{"id": uid, "client_code": req.ClientCode, "message": "Client created. The account email could not be sent."})
		return
	}
	if err := utility.SendOTPEmail(admin.Email, emailTemplate); err != nil {
		logger.Log("ERROR", "CreateClient provision mail: "+err.Error())
		c.JSON(http.StatusCreated, gin.H{"id": uid, "client_code": req.ClientCode, "message": "Client created. The account email could not be sent."})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": uid, "client_code": req.ClientCode, "message": "Client created. A login email was sent to the client admin."})
}

func ListClientAdmins(c *gin.Context) {
	usersPtr, err := db.GetUsersByRole(roleAdmin)
	if err != nil {
		logger.Log("ERROR", "ListClientAdmins: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	type respUser struct {
		UserId     string    `json:"user_id"`
		Username   string    `json:"username"`
		Role       string    `json:"role"`
		Email      string    `json:"email"`
		ClientCode string    `json:"client_code"`
		CreatedAt  time.Time `json:"created_at"`
	}
	list := make([]respUser, 0, len(*usersPtr))
	for _, u := range *usersPtr {
		list = append(list, respUser{
			UserId:     u.ID,
			Username:   u.UserName,
			Role:       u.AccessType,
			Email:      u.Email,
			ClientCode: u.ClientID,
			CreatedAt:  u.CreatedAt,
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
