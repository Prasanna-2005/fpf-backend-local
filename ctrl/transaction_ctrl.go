package ctrl

import (
	"errors"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/modules"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"github.com/gin-gonic/gin"
)

const (
	FilingNotFound = "No filing found for the provided filing_id"
	EmptyFiling    = "Invalid empty filing id"
	Modified       = "Changes accepted successfully"
	NotConfirmed   = "Line item still in edit mode"
	Accepted       = "Your values accepted successfully"
	LineItemErr    = "Line item type cannot be empty. It should NULL if no datatype present"
	EmptyTableErr  = "No data present in table data but has_table_data marked true"
)

func AcceptItemValue(c *gin.Context) {
	filingID := c.Param("filing_id")
	fundId := c.Param("fund_id")
	if filingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": EmptyFiling})
		return
	}

	var payload *models.FieldItem
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
	if strings.TrimSpace(payload.FieldItemType) == "" {
		logger.Log("WARNING", "Field item type cannot be empty. It should NULL if no datatype present")
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	if len(payload.FieldData) != 0 {
		for _, lineItem := range payload.FieldData {
			if strings.TrimSpace(lineItem.LineItemType) == "" {
				logger.Log("WARNING", LineItemErr)
			}
			if lineItem.HasTableData && reflect.ValueOf(payload.TableData).Len() == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"message": LineItemErr})
				return
			}
		}
	}
	if payload.HasTableData && reflect.ValueOf(payload.TableData).Len() == 0 {
		logger.Log("WARNING", EmptyTableErr)
		c.JSON(http.StatusBadRequest, gin.H{"message": EmptyTableErr})
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

	resp, err := modules.BuildResponselegacy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}



func GetApprovedFilingResponse(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("filing_id"), 10, 64)
	if err != nil {
		logger.Log("ERROR", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}	
	//response
	resp, err := modules.TransformToSimpleResponse(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func RevertLatest(c *gin.Context) {
	filingID := c.Param("filing_id")
	fundID := c.Param("fund_id")
	if filingID == "" || fundID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "filing_id and fund_id are required"})
		return
	}

	var payload models.FieldItem
	if err := c.ShouldBindJSON(&payload); err != nil {
		logger.Log("ERROR", "Payload error: "+err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	if !payload.Confirmed || payload.Editing {
		logger.Log("INFO", "line item still in edit mode")
		c.JSON(http.StatusUnprocessableEntity, gin.H{"message": "line item not confirmed or still in edit mode"})
		return
	}
	if strings.TrimSpace(payload.FieldItemType) == "" {
		logger.Log("WARNING", "Field item type cannot be empty. It should NULL if no datatype present")
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}

	if len(payload.FieldData) != 0 {
		for _, lineItem := range payload.FieldData {
			if strings.TrimSpace(lineItem.LineItemType) == "" {
				logger.Log("WARNING", LineItemErr)
			}
			if lineItem.HasTableData && reflect.ValueOf(payload.TableData).Len() == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"message": LineItemErr})
				return
			}
		}
	}
	if payload.HasTableData && reflect.ValueOf(payload.TableData).Len() == 0 {
		logger.Log("WARNING", EmptyTableErr)
		c.JSON(http.StatusBadRequest, gin.H{"message": EmptyTableErr})
		return
	}
	userID := c.GetHeader("X-User-Id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "missing user ID"})
		return
	}

	err := db.RevertLineItem(&payload, filingID, userID, fundID)

	switch {
	case errors.Is(err, db.ErrFilingNotFound):
		c.JSON(http.StatusNotFound, gin.H{"message": "filing not found"})
	case errors.Is(err, db.ErrNothingToDo):
		c.JSON(http.StatusOK, gin.H{"message": "no changes to update"})
	case err != nil:
		logger.Log("ERROR", "RevertLatest error: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "internal server error"})
	default:
		c.JSON(http.StatusOK, gin.H{"message": "line item updated successfully"})
	}
}
