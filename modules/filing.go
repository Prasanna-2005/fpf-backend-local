// modules/filing.go
package modules

import (
	"errors"
	"fmt"
	"form_pf_backend/db"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

const maxUploadBytes = 60_000_000

// UploadRequest represents the structure of an upload request
type UploadRequest struct {
	UserID     string                  `form:"user_id" binding:"required"`
	NewFiling  bool                    `form:"new_filing"`
	FilingName string                  `form:"filing_name" binding:"required"`
	Timestamp  time.Time               `form:"timestamp" time_format:"2006-01-02T15:04:05.000Z" binding:"required"`
	Files      []*multipart.FileHeader `form:"files" binding:"required"`
}

var (
	errInvalidFile = errors.New("invalid file")
	dirOnce        sync.Map
	dirErrors      sync.Map
	allowedExts    = map[string]bool{".zip": true, ".xls": true, ".xlsx": true, ".pdf": true}
)

// InitVolume creates the root directory (left for backward-compatibility)
func InitVolume(root string) { _ = os.MkdirAll(root, 0755) }

// ValidateAndBind validates and binds the incoming request
func ValidateAndBind(c *gin.Context) (*UploadRequest, bool) {
	logger.Log("INFO", "[ValidateAndBind] Starting request validation")

	if err := c.Request.ParseMultipartForm(maxUploadBytes); err != nil {
		handleParseError(c, err)
		return nil, false
	}

	var req UploadRequest
	if err := c.ShouldBind(&req); err != nil {
		handleBindError(c, err)
		return nil, false
	}

	if len(req.Files) == 0 {
		logger.Log("WARNING", "[ValidateAndBind] No files provided")
		c.JSON(http.StatusBadRequest, gin.H{"message": "no files provided"})
		return nil, false
	}

	logger.Log("INFO", fmt.Sprintf("[ValidateAndBind] Successfully validated request with %d files", len(req.Files)))
	return &req, true
}

// ProcessUpload handles the main upload business logic
func ProcessUpload(c *gin.Context, root string, req *UploadRequest) error {
	clientID, _ := c.Get("clientId")
	clientDir := filepath.Join(root, fmt.Sprintf("%v", clientID))

	logger.Log("INFO", fmt.Sprintf("[ProcessUpload] Processing upload - client=%v, filing=%s, new=%t, files=%d",
		clientID, req.FilingName, req.NewFiling, len(req.Files)))

	if err := EnsureVolumeDir(clientDir); err != nil {
		logger.Log("ERROR", fmt.Sprintf("[ProcessUpload] Failed to create client directory %s: %v", clientDir, err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create client directory"})
		return err
	}

	exists := filingExists(clientDir, sanitize(req.FilingName))
	if err := validateFilingConstraints(req.NewFiling, exists, req.FilingName); err != nil {
		handleFilingValidationError(c, err, req.NewFiling)
		return err
	}

	targetDir := filepath.Join(clientDir, sanitize(req.FilingName))
	if err := EnsureVolumeDir(targetDir); err != nil {
		logger.Log("ERROR", fmt.Sprintf("[ProcessUpload] Failed to create target directory %s: %v", targetDir, err))
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create directory"})
		return err
	}

	if err := saveAndExtract(req.Files, targetDir); err != nil {
		handleSaveError(c, targetDir, err)
		return err
	}

	logger.Log("INFO", fmt.Sprintf("[ProcessUpload] Successfully processed upload for filing: %s", req.FilingName))
	c.JSON(http.StatusOK, gin.H{"message": "Successfully uploaded", "filing_name": sanitize(req.FilingName)})
	return nil
}

// Helper functions
func handleParseError(c *gin.Context, err error) {
	if errors.Is(err, http.ErrNotMultipart) {
		logger.Log("WARNING", "[ValidateAndBind] Request is not multipart")
		c.JSON(http.StatusBadRequest, gin.H{"message": "not multipart"})
	} else {
		logger.Log("WARNING", fmt.Sprintf("[ValidateAndBind] Request entity too large: %v", err))
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"message": "request entity too large"})
	}
}

func handleBindError(c *gin.Context, err error) {
	logger.Log("WARNING", fmt.Sprintf("[ValidateAndBind] Binding failed: %v", err))
	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		c.JSON(http.StatusBadRequest, gin.H{"message": verr[0].Error()})
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	}
}

func validateFilingConstraints(newFiling, exists bool, filingName string) error {
	if newFiling && exists {
		return fmt.Errorf("duplicate filing: %s", filingName)
	}
	if !newFiling && !exists {
		return fmt.Errorf("missing filing: %s", filingName)
	}
	return nil
}

func handleFilingValidationError(c *gin.Context, err error, newFiling bool) {
	if newFiling {
		logger.Log("WARNING", fmt.Sprintf("[ProcessUpload] %v", err))
		c.JSON(http.StatusConflict, gin.H{"message": "filing name already exists"})
	} else {
		logger.Log("WARNING", fmt.Sprintf("[ProcessUpload] %v", err))
		c.JSON(http.StatusBadRequest, gin.H{"message": "filing doesn't exist"})
	}
}

func handleSaveError(c *gin.Context, targetDir string, err error) {
	logger.Log("ERROR", fmt.Sprintf("[ProcessUpload] Save and extract failed: %v", err))

	if cleanupErr := os.RemoveAll(targetDir); cleanupErr != nil {
		logger.Log("WARNING", fmt.Sprintf("[ProcessUpload] Failed to cleanup directory %s: %v", targetDir, cleanupErr))
	} else {
		logger.Log("INFO", fmt.Sprintf("[ProcessUpload] Cleaned up directory: %s", targetDir))
	}

	if errors.Is(err, errInvalidFile) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid file format"})
	} else {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Internal Server Error"})
	}
}
func RemoveDir(dir string) error {
	if dir == "" || dir == "." || dir == ".." || dir == "/" {
		return errors.New("refusing to delete risky path: " + dir)
	}

	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf("cannot stat %q: %w", dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove %q: %w", dir, err)
	}
	return nil
}

func filingExists(clientDir, filingName string) bool {
	recPresent := false
	allfilings, err := db.GetAllFilings()
	if err != nil {
		logger.Log("ERROR", "Db error: "+err.Error())
		return true
	}
	for _, filing := range *allfilings {
		if filing.FilingName == filingName {
			recPresent = true
		}
	}
	targetDirName := sanitize(filingName)
	entries, err := os.ReadDir(clientDir)
	if err != nil {
		logger.Log("WARNING", fmt.Sprintf("[filingExists] Failed to read directory %s: %v", clientDir, err))
		return false
	}

	for _, e := range entries {
		if e.IsDir() && e.Name() == targetDirName && recPresent {
			logger.Log("INFO", fmt.Sprintf("[filingExists] Found existing filing: %s", e.Name()))
			return true
		} else if e.IsDir() && e.Name() == targetDirName && !recPresent {
			brokenDir := filepath.Join(clientDir, targetDirName)
			err := RemoveDir(brokenDir)
			if err != nil {
				logger.Log("ERROR", fmt.Sprintf("Incomplete filing Directory not removed:%v", brokenDir))
				logger.Log("ERROR", "The above error occured :"+err.Error())
				return true
			}
			fmt.Println(err)
			logger.Log("INFO", fmt.Sprintf("Incomplete filing Directory removed:%v", filingName))
		}
	}

	return false
}

func sanitize(name string) string {
	sanitized := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
	logger.Log("INFO", fmt.Sprintf("[sanitize] '%s' -> '%s'", name, sanitized))
	return sanitized
}

func saveAndExtract(files []*multipart.FileHeader, dir string) error {
	logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Processing %d files for directory: %s", len(files), dir))

	for i, fh := range files {
		logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Processing file %d/%d: %s (size: %d bytes)",
			i+1, len(files), fh.Filename, fh.Size))

		if !allowedExt(fh.Filename) {
			logger.Log("WARNING", fmt.Sprintf("[saveAndExtract] Rejected file with invalid extension: %s", fh.Filename))
			return errInvalidFile
		}

		dst := filepath.Join(dir, fh.Filename)
		if err := saveUploadedFile(fh, dst); err != nil {
			logger.Log("ERROR", fmt.Sprintf("[saveAndExtract] Failed to save file %s: %v", fh.Filename, err))
			return err
		}

		// Extract ZIP files to their own subdirectory (key change)
		if strings.EqualFold(filepath.Ext(fh.Filename), ".zip") {
			zipDir := filepath.Join(dir, strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename)))
			logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Starting ZIP extraction for: %s to %s", dst, zipDir))

			if err := ExtractAndValidateZip(dst, zipDir); err != nil {
				logger.Log("ERROR", fmt.Sprintf("[saveAndExtract] ZIP extraction failed for %s: %v", dst, err))
				return err
			}
			logger.Log("INFO", fmt.Sprintf("[saveAndExtract] ZIP extraction completed successfully for: %s", dst))
		}
	}

	logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Successfully processed all %d files", len(files)))
	return nil
}

func saveUploadedFile(src *multipart.FileHeader, dst string) error {
	in, err := src.Open()
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer out.Close()

	bytesWritten, err := io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	logger.Log("INFO", fmt.Sprintf("[saveUploadedFile] Copied %d bytes to %s", bytesWritten, dst))
	return nil
}

func RemoveDir(dir string) error {
	// sanity check: refuse to delete “/”, “.”, “..”, “”, etc.
	if dir == "" || dir == "." || dir == ".." || dir == "/" {
		return errors.New("refusing to delete risky path: " + dir)
	}

	//make sure it exists and is a directory
	if info, err := os.Stat(dir); err != nil {
		return fmt.Errorf("cannot stat %q: %w", dir, err)
	} else if !info.IsDir() {
		return fmt.Errorf("%q is not a directory", dir)
	}

	// Actual deletion
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("failed to remove %q: %w", dir, err)
	}
	return nil
}

func allowedExt(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	allowed := allowedExts[ext]
	if !allowed {
		logger.Log("INFO", fmt.Sprintf("[allowedExt] Extension '%s' not allowed for file: %s", ext, filename))
	}
	return allowed
}

// EnsureVolumeDir ensures a directory exists, creating it if necessary
func EnsureVolumeDir(path string) error {
	key := filepath.Clean(path)
	actualOnce, _ := dirOnce.LoadOrStore(key, &sync.Once{})
	once := actualOnce.(*sync.Once)

	var firstErr error
	once.Do(func() {
		logger.Log("INFO", fmt.Sprintf("[EnsureVolumeDir] Creating directory: %s", key))
		firstErr = os.MkdirAll(key, 0755)
		if firstErr != nil {
			dirErrors.Store(key, firstErr)
			logger.Log("ERROR", fmt.Sprintf("[EnsureVolumeDir] Failed to create directory %s: %v", key, firstErr))
		} else {
			logger.Log("INFO", fmt.Sprintf("[EnsureVolumeDir] Successfully created directory: %s", key))
		}
	})

	if e, ok := dirErrors.Load(key); ok {
		return e.(error)
	}
	return nil
}
