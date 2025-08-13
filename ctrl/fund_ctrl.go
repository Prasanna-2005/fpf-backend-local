package ctrl

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetFunds(c *gin.Context) {
	fund, err := db.GetAllfunds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": ServerError,
		})
		return
	}
}
