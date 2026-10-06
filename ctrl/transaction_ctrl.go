package ctrl

import (
	"errors"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"form_pf_backend/modules"
	"net/http"
	"reflect"
	"strings"
	"time"

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
	filingID, fundId := c.Param("filing_id"), c.Param("fund_id")
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
	id := c.Param("filing_id")
	if id == "" {
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
	id := c.Param("filing_id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}

	if qnos, ok := modules.MandateFieldCheck(id); !ok {
		logger.Log("WARNING", fmt.Sprintf("Missing mandatory fields:\n%v", strings.Join(qnos, "\n")))
		c.JSON(http.StatusBadRequest, gin.H{"message": qnos})
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

func CancelFiling(uploadRoot string) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIDVal, ok := c.Get("clientId")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			return
		}
		clientID := clientIDVal.(string)

		var req struct {
			FilingName string `json:"filing_name" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "filing_name is required"})
			return
		}

		err := modules.CancelDraftFiling(
			uploadRoot,
			clientID,
			req.FilingName,
		)

		switch {
		case err == nil:
			c.JSON(http.StatusOK, gin.H{
				"message":     "filing cancelled successfully",
				"filing_name": req.FilingName,
			})

		case errors.Is(err, errors.New("filing already submitted")):
			c.JSON(http.StatusConflict, gin.H{
				"message": "filing already submitted and cannot be cancelled",
			})

		case errors.Is(err, errors.New("draft filing not found")):
			c.JSON(http.StatusNotFound, gin.H{
				"message": "draft filing not found",
			})

		default:
			logger.Log("ERROR", "CancelFiling: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "failed to cancel filing",
			})
		}
	}
}

func GetCompLogs(c *gin.Context) {
	fid := c.Param("filing_id")
	if fid == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid filingId"})
		return
	}
	id, _ := c.Get("clientId")
	cid, _ := id.(string)
	series, err := db.GetSeries(fid, cid)
	if err != nil {
		logger.Log("ERROR", err.Error())
		c.JSON(http.StatusNotFound, gin.H{"message": ServerError})
		return
	}
	resp, err := modules.BuildComp(fid, series, cid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

type HistoryResponse struct {
	FundSeriesid   string    `json:"fund_series_id"`
	FieldNo        string    `json:"field_no"`
	Value          string    `json:"value"`
	SourceFiles    []string  `json:"source_files"`
	ManuallyEdited bool      `json:"manually_edited"`
	LineText       string    `json:"line_text"`
	IsCurrentValue bool      `json:"is_current_value"`
	CreatedAt      time.Time `json:"created_at"`
	CreatedBy      string    `json:"created_by"`
	UpdatedAt      time.Time `json:"updated_at"`
	UpdatedBy      string    `json:"updated_by"`
}

func GetFieldHistory(c *gin.Context) {
	filingID := c.Query("filingid")
	fundSeriesID := c.Query("fundSeriesid")
	fieldNo := c.Query("fieldNo")

	var missing []string
	if filingID == "" {
		missing = append(missing, "filingid")
	}
	if fundSeriesID == "" {
		missing = append(missing, "fundSeriesid")
	}
	if fieldNo == "" {
		missing = append(missing, "fieldNo")
	}
	if len(missing) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing required parameter(s): " + strings.Join(missing, ", "),
		})
		return
	}
	logger.Log(
		"DEBUG",
		fmt.Sprintf("Fetching history for FilingID: %s, FundSeriesID: %s, FieldNo: %s",
			filingID, fundSeriesID, fieldNo,
		),
	)

	rows, err := db.GetFieldHistoryDB(filingID, fundSeriesID, fieldNo)
	if err != nil {
		logger.Log("ERROR", "GetFieldHistory: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": ServerError})
		return
	}

	groupedResponse := make(map[string][]HistoryResponse)
	for _, r := range rows {
		files := models.ParseSourceFiles(r.SourceFiles)
		// A saved edit has updatedBy and no source files. A revert also sets
		// updatedBy, on the extracted row, so that row can still show its files.
		item := HistoryResponse{
			FundSeriesid:   r.FundSeriesid,
			FieldNo:        r.FieldNo,
			Value:          r.Value,
			SourceFiles:    files,
			ManuallyEdited: strings.TrimSpace(r.UpdatedByID) != "" && len(files) == 0,
			LineText:       r.LineText,
			IsCurrentValue: r.IsCurrentValue,
			CreatedAt:      r.CreatedAt,
			CreatedBy:      r.CreatedBy,
			UpdatedAt:      r.UpdatedAt,
			UpdatedBy:      r.UpdatedBy,
		}
		groupedResponse[r.FormLineNo] = append(groupedResponse[r.FormLineNo], item)
	}

	c.JSON(http.StatusOK, groupedResponse)
}
