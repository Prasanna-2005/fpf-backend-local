package ctrl

import (
	"form_pf_backend/db"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func GetFilings(c *gin.Context) {
	filingsPtr, err := db.GetAllFilings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": ServerError,
		})
		return
	}

	// success payload
	type respFiling struct {
		ID          uint      `json:"id"`
		FilingName  string    `json:"filing_name"`
		IsSubmitted bool      `json:"is_submitted"`
		IsDeleted   bool      `json:"is_deleted"`
		CreatedAt   time.Time `json:"created_at"`
		CreatedBy   int       `json:"created_by"`
		UpdatedAt   time.Time `json:"updated_at,omitempty"`
		UpdatedBy   int       `json:"updated_by,omitempty"`
	}

	var list []respFiling
	for _, f := range *filingsPtr {
		list = append(list, respFiling{
			ID:          f.ID,
			FilingName:  f.FilingName,
			IsSubmitted: f.IsSubmitted,
			IsDeleted:   f.IsDeleted,
			CreatedAt:   f.CreatedAt,
			CreatedBy:   f.CreatedBy,
			UpdatedAt:   f.UpdatedAt,
			UpdatedBy:   f.UpdatedBy,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": list})
}
