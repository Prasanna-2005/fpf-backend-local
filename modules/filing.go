package modules

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const maxUploadBytes = 50 << 20

type uploadRequest struct {
	UserID     string                  `form:"user_id" binding:"required"`
	FilingID   *string                 `form:"filing_id"`
	FilingName string                  `form:"filing_name" binding:"required"`
	Timestamp  time.Time               `form:"timestamp" time_format:"2006-01-02T15:04:05.000Z" binding:"required"`
	Files      []*multipart.FileHeader `form:"files" binding:"required"`
}

func FilingExists(root, clientID, filingName string) bool {
	clientDir := filepath.Join(root, clientID)
	_ = os.MkdirAll(clientDir, 0755) // ensure dir exists

	suffix := "+" + sanitizeName(filingName)
	entries, _ := os.ReadDir(clientDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), suffix) {
			return true
		}
	}
	return false
}

func sanitizeName(name string) string {
	return strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
}

func SaveUploadedFile(srcFile interface {
	Open() (multipart.File, error)
}, dst string) error {
	src, err := srcFile.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, src)
	return err
}

func ValidateRequest(c *gin.Context) (*uploadRequest, bool) {
	if err := c.Request.ParseMultipartForm(maxUploadBytes); err != nil {
		c.JSON(http.StatusRequestEntityTooLarge,
			gin.H{"message": "request entity too large"})
		return nil, false
	}

	var req uploadRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid file format"})
		return nil, false
	}
	return &req, true
}

func ResolveFilingDir(c *gin.Context, root, clientID string, req *uploadRequest) (string, bool) {
	dirName := func() string {
		if req.FilingID != nil && *req.FilingID != "" {
			return *req.FilingID + "+" + sanitizeName(req.FilingName)
		}
		// fresh upload
		if FilingExists(root, clientID, req.FilingName) {
			c.JSON(http.StatusConflict, gin.H{"message": "filing name already exists"})
			return ""
		}
		return fmt.Sprintf("fil_%d+%s", time.Now().Unix(), sanitizeName(req.FilingName))
	}()

	if dirName == "" {
		return "", false
	}

	filingDir := filepath.Join(root, clientID, dirName)
	_ = os.MkdirAll(filingDir, 0755)
	return filingDir, true
}

func SaveFiles(files []*multipart.FileHeader, dir string) error {
	for _, fh := range files {
		dst := filepath.Join(dir, fh.Filename)
		if err := SaveUploadedFile(fh, dst); err != nil {
			return err
		}
	}
	return nil
}
