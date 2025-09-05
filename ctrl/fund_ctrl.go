package ctrl

import (
	"form_pf_backend/db"
	"form_pf_backend/models"
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetFunds(c *gin.Context) {
	type successResp struct {
		Funds []models.Fund `json:"funds"`
	}
	funds, err := db.GetAllFunds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Internal Server Error. Please reach out to your admin",
		})
		return
	}

	if len(funds) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "Fund ID does not exist",
		})
		return
	}

	c.JSON(http.StatusOK, successResp{Funds: funds})
}
