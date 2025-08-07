package ctrl

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type NewUserRequest struct {
	Data struct {
		Username  string `json:"username" binding:"required"`
		Password  string `json:"password" binding:"required"`
		UserRole  string `json:"user_role" binding:"required"`
		CreatedAt string `json:"created_at"` // ISO-8601
	} `json:"data"`
	Role       string `json:"role"`
	AdminID    string `json:"admin_id"`
	ClientCode string `json:"client_code"`
}

func NewUser(c *gin.Context) {
	var req NewUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

}

func GetUsers(c *gin.Context) {

}

func DeleteUser(c *gin.Context) {

}
