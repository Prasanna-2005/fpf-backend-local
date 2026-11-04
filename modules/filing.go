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
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

const (
	maxUploadBytes = 60_000_000
	copyBufferSize = 1024 * 1024 // 1MB buffer
)

// UploadRequest represents the structure of an upload request
type UploadRequest struct {
	UserID     string                  `form:"user_id" binding:"required"`
	NewFiling  bool                    `form:"new_filing"`
	FilingName string                  `form:"filing_name" binding:"required,max=255"`
	Timestamp  time.Time               `form:"timestamp" time_format:"2006-01-02T15:04:05.000Z" binding:"required"`
	Files      []*multipart.FileHeader `form:"files" binding:"required"`
}

var (
	errInvalidFile      = errors.New("invalid file")
	DuplicateFileUpload = errors.New("Duplicate filenames found in upload")
	dirOnce             sync.Map
	dirErrors           sync.Map
	allowedExts         = map[string]bool{".zip": true, ".xls": true, ".xlsx": true, ".pdf": true}
	directory           = make(map[string][]string) // contains all base filenames.ext present in the directory
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

func filingExistsInDB(filingName string) (bool, error) {
	_, err := db.GetFilingTrackerByName(filingName)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		logger.Log("ERROR", "DB error: "+err.Error())
		return false, err
	}
	return true, nil
}

func ProcessUpload(c *gin.Context, root string, req *UploadRequest) error {
	var skipped, skippedFileRelPath []string
	clientID, _ := c.Get("clientId")

	clientDir := filepath.Join(root, fmt.Sprintf("%v", clientID))
	filingName := sanitize(req.FilingName)
	targetDir := filepath.Join(clientDir, filingName)

	logger.Log(
		"INFO",
		fmt.Sprintf(
			"[ProcessUpload] client=%v filing=%s new=%t files=%d",
			clientID, filingName, req.NewFiling, len(req.Files),
		),
	)

	//  Ensure client directory exists
	if err := EnsureVolumeDir(clientDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create client directory"})
		return err
	}

	//  DB-only validation
	existsInDB, err := filingExistsInDB(req.FilingName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "database error"})
		return err
	}

	if req.NewFiling && existsInDB {
		c.JSON(http.StatusConflict, gin.H{"message": "filing already exists"})
		return fmt.Errorf("duplicate filing")
	}

	//  ALWAYS create (or recreate) filing directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create filing directory"})
		return err
	}

	//  Save & extract files
	if err := saveAndExtract(req.Files, targetDir, &skipped, filingName, req.NewFiling); err != nil {
		// Cleanup on failure
		_ = os.RemoveAll(targetDir)
		handleSaveError(c, targetDir, err)
		return err
	}
	for _, filePath := range skipped {
		skippedFileRelPath = append(skippedFileRelPath, getOrgName(filepath.Base(filePath)))
	}

	if len(skipped) > 0 {
		c.JSON(http.StatusConflict, gin.H{
			"message":       "Duplicate filename uploaded in mapped directory",
			"filing_name":   filingName,
			"skipped_files": skippedFileRelPath,
		})
		return DuplicateFileUpload
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Successfully uploaded",
		"filing_name": filingName,
	})

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

// func RemoveDir(dir string) error {
// 	if dir == "" || dir == "." || dir == ".." || dir == "/" {
// 		return errors.New("refusing to delete risky path: " + dir)
// 	}

// 	if info, err := os.Stat(dir); err != nil {
// 		return fmt.Errorf("cannot stat %q: %w", dir, err)
// 	} else if !info.IsDir() {
// 		return fmt.Errorf("%q is not a directory", dir)
// 	}

// 	if err := os.RemoveAll(dir); err != nil {
// 		return fmt.Errorf("failed to remove %q: %w", dir, err)
// 	}
// 	return nil
// }

func sanitize(name string) string {
	sanitized := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
	logger.Log("INFO", fmt.Sprintf("[sanitize] '%s' -> '%s'", name, sanitized))
	return sanitized
}

func directoryFiles(dir string) {
	xlsxFiles, pdfFiles, err := findFiles(dir)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("failed to find files in %s: %v", dir, err))
	}
	files := append(xlsxFiles, pdfFiles...)
	key := filepath.Base(dir)
	for i, path := range files {
		files[i] = getOrgName(filepath.Base(path))
	}
	directory[key] = files
}

func saveAndExtract(files []*multipart.FileHeader, dir string, skippedFiles *[]string, filingName string, newFiling bool) error {
	var skipped []string
	logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Processing %d files for directory: %s", len(files), dir))
	directoryFiles(dir)
	for i, fh := range files {
		logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Processing file %d/%d: %s (size: %d bytes)",
			i+1, len(files), fh.Filename, fh.Size))

		if !allowedExt(fh.Filename) {
			logger.Log("WARNING", fmt.Sprintf("[saveAndExtract] Rejected file with invalid extension: %s", fh.Filename))
			return errInvalidFile
		}
		// UUID prefixed (not suffixed) so original ext stays at the end — findFiles() relies on filepath.Ext()
		uuidName := generateUUIDName(fh.Filename)
		dst := filepath.Join(dir, uuidName)
		if checkDuplicateFilenames(dst, filingName) && newFiling {
			logger.Log("WARNING", "Duplicate file presence in upload/File permission error")
			skipped = append(skipped, dst)
			continue
		}

		if err := saveUploadedFile(fh, dst); err != nil {
			logger.Log("ERROR", fmt.Sprintf("[saveAndExtract] Failed to save file %s: %v", fh.Filename, err))
			return err
		}

		// Extract ZIP files to their own subdirectory (key change)
		if strings.EqualFold(filepath.Ext(fh.Filename), ".zip") {
			dst = filepath.Join(dir, fh.Filename)
			zipDir := filepath.Join(dir, strings.TrimSuffix(fh.Filename, filepath.Ext(fh.Filename)))
			logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Starting ZIP extraction for: %s to %s", dst, zipDir))

			if err := ExtractAndValidateZip(dst, zipDir, filingName, &skipped, newFiling); err != nil {
				logger.Log("ERROR", fmt.Sprintf("[saveAndExtract] ZIP extraction failed for %s: %v", dst, err))
				return err
			}
			logger.Log("INFO", fmt.Sprintf("[saveAndExtract] ZIP extraction completed successfully for: %s", dst))
		}
	}
	*skippedFiles = skipped
	logger.Log("INFO", fmt.Sprintf("[saveAndExtract] Successfully processed  %d files", len(files)))
	return nil
}

// should check only filename at any directory level and flag duplicate files
func checkDuplicateFilenames(dst, filingName string) bool {
	filename := filepath.Base(dst)
	if slices.Contains(directory[filingName], getOrgName(filename)) {
		logger.Log("WARNING", fmt.Sprintf("Duplicate filename:%v found in upload", getOrgName(filename)))
		return true
	}
	return false
}

func saveUploadedFile(src *multipart.FileHeader, dst string) error {
	if strings.EqualFold(filepath.Ext(dst), ".zip") {
		orig := getOrgName(filepath.Base(dst))
		dst = filepath.Join(filepath.Dir(dst), orig) // removing uuid in zip and preserving it's original name
	}
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

	buf := make([]byte, copyBufferSize)
	bytesWritten, err := io.CopyBuffer(out, in, buf)
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

// validates only allowed file extension
// current support( xlsx,xls,pdf)
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

// deletes dir
func CancelDraftFiling(uploadRoot, clientID, filingName string) error {
	if strings.TrimSpace(filingName) == "" {
		return errors.New("empty filing name")
	}

	// Check if filing is already submitted (DB)
	allfilings, err := db.GetAllFilings()
	if err != nil {
		return fmt.Errorf("db error: %w", err)
	}

	for _, filing := range *allfilings {
		if filing.FilingName == filingName {
			//  Submitted filings cannot be cancelled
			return errors.New("filing already submitted")
		}
	}

	// Draft directory path (only this folder)
	targetDir := filepath.Join(
		uploadRoot,
		clientID,
		sanitize(filingName),
	)

	//  If draft directory does NOT exist → treat as already cancelled
	if _, err := os.Stat(targetDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat draft directory: %w", err)
	}

	//  Safe delete of ONLY this filing folder
	if err := RemoveDir(targetDir); err != nil {
		return fmt.Errorf("failed to delete draft filing: %w", err)
	}

	return nil
}
