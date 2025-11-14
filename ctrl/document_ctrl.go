// ctrl/document_ctrl.go
package ctrl

import (
	"fmt"
	"net/http"
	"sort"

	"form_pf_backend/db"
	"form_pf_backend/ipc"
	"form_pf_backend/models"
	"form_pf_backend/modules"
	"form_pf_backend/utility"

	"github.com/gin-gonic/gin"
)

const (
	FileSubmitted = "Files submitted successfully"
	NoFilingName  = "Filing Name does not exist"
)

// UploadDoc handles document upload requests with optimized flow
func UploadDoc(volumeRoot string) gin.HandlerFunc {
	return func(c *gin.Context) {
		logger.Log("DEBUG", fmt.Sprintf("UploadDoc invoked, volumeRoot=%s", volumeRoot))

		// Pipeline approach: each step returns early on failure
		if !ensureUploadDirectory(c, volumeRoot) {
			return
		}

		req, ok := modules.ValidateAndBind(c)
		if !ok {
			return // ValidateAndBind already handled the response
		}

		logger.Log("INFO", fmt.Sprintf("Validated request: user_id=%s, new_filing=%t, filing_name=%s",
			req.UserID, req.NewFiling, req.FilingName))

		if err := modules.ProcessUpload(c, volumeRoot, req); err != nil {
			logger.Log("ERROR", fmt.Sprintf("ProcessUpload failed: %v", err))
			return // ProcessUpload already handled the response
		}

		logger.Log("INFO", fmt.Sprintf("Upload successful for filing_name=%s", req.FilingName))
	}
}

// ensureUploadDirectory ensures the upload directory exists
func ensureUploadDirectory(c *gin.Context, volumeRoot string) bool {
	if err := modules.EnsureVolumeDir(volumeRoot); err != nil {
		logger.Log("ERROR", fmt.Sprintf("EnsureVolumeDir failed: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create upload directory"})
		return false
	}
	logger.Log("DEBUG", "Upload volume directory OK")
	return true
}

// MapFiles is now a standard gin.HandlerFunc
func MapFiles(c *gin.Context) {
	var req models.FileIdentificationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	var cid string
	cid = "system"
	temp, ok := c.Get("clientId")
	if !ok {
		logger.Log("WARNING", "clientId not found")
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	cid = temp.(string)
	resp, err := modules.IdentifyFiles(&req, cid)

	if resp != nil && len(resp.FileMappings) > 0 {
		sort.Slice(resp.FileMappings, func(i, j int) bool {
			if resp.FileMappings[i].FundName == resp.FileMappings[j].FundName {
				return resp.FileMappings[i].FileType < resp.FileMappings[j].FileType
			}
			return resp.FileMappings[i].FundName < resp.FileMappings[j].FundName
		})
		sort.Slice(resp.FundDetails, func(i, j int) bool {
			if resp.FundDetails[i].FundName == resp.FundDetails[j].FundName {
				return resp.FundDetails[i].FileType < resp.FundDetails[j].FileType
			}
			return resp.FundDetails[i].FundName < resp.FundDetails[j].FundName
		})

	}
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("mapper err: %v", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func SubmitFiles(c *gin.Context) {
	var req models.SubmitReq
	var cid = "system"
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": PayloadError})
		return
	}
	temp, ok := c.Get("clientId")
	if !ok {
		logger.Log("WARNING", "No client id found")
	}

	cid, ok = temp.(string)

	if !ok {
		logger.Log("WARNING", "clientid parsing error")
	}
	uid := req.UserID

	if req.NewFiling {
		filingId, err := db.CreateTracker(&req, cid)
		if err != nil {
			logger.Log("ERROR", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
			return
		}
		mappers, err := db.CreateMapper(&req, filingId)
		if err != nil {
			logger.Log("ERROR", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
			return
		}
		err = db.BatchSaveFileMappings(mappers)
		if err != nil {
			logger.Log("ERROR", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
			return
		}
		err = ipc.TransferSignal(utility.CreateSignal(uid, filingId, req.Series))
		if err != nil {
			logger.Log("ERROR", "Service error: "+err.Error())
		}
		c.JSON(http.StatusOK, gin.H{"message": FileSubmitted, "filing_id": filingId})
		return
	}

	tracker, err := db.GetFilingTrackerByName(req.FilingName)
	if err != nil {
		logger.Log("ERROR", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": NoFilingName})
	}
	err = db.UpdateTracker(tracker, &req)
	if err != nil {
		logger.Log("ERROR", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
	}
	fmt.Println(req)
	err = db.UpdateMapper(tracker.Id, &req)
	if err != nil {
		logger.Log("ERROR", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": ServerError})
	}

	err = ipc.TransferSignal(utility.CreateSignal(uid, tracker.Id, req.Series))
	if err != nil {
		logger.Log("ERROR", "Service error: "+err.Error())
	}
	c.JSON(http.StatusOK, gin.H{"message": FileSubmitted, "filing_id": tracker.Id})
}

// func Cancel(c *gin.Context) {
// 	var payload struct {
// 		Cancel     string `json:"cancel" binding:"required"`
// 		FilingName string `json:"filing_name" binding:"required"`
// 	}

// 	if !payload.Cancel {
// 		logger.Log("WARNING", "Cancellation not confirmed")
// 	}
// }
