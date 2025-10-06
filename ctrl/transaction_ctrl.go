package ctrl

import (
	"errors"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/modules"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	FilingNotFound = "No filing found for the provided filing_id"
	EmptyFiling    = "Invalid empty filing id"
	Modified       = "Changes accepted successfully"
	NotConfirmed   = "Line item still in edit mode"
	Accepted       = "Your values accepted successfully"
)

func AcceptItemValue(c *gin.Context) {
	filingID := c.Param("filing_id")
	fundId := c.Param("fund_id")
	if filingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": EmptyFiling})
		return
	}

	var payload *models.ResponseField
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "Payload error:"+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	if !payload.Confirmed || payload.Editing {
		logger.Log("INFO", "line item still in edit mode")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": NotConfirmed})
		return
	}
	userID := c.GetHeader("X-User-Id")

	err := db.UpdateLineItem(payload, filingID, userID, fundId)

	switch {
	case errors.Is(err, db.ErrFilingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"message": FilingNotFound})
	case errors.Is(err, db.ErrNothingToDo):
		c.JSON(http.StatusOK, gin.H{"message": Accepted})
	case err != nil:
		logger.Log("ERROR", "AcceptItem: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
	default:
		c.JSON(http.StatusOK, gin.H{"message": Modified})
	}

}

func GetFilingResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("filing_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}

	resp, err := modules.BuildResponse(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}



func GetApprovedFilingResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("filing_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}

	resp, err := modules.TransformToSimpleResponse(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}