package modules

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func HandleFilingZipDownload(c *gin.Context, root string) {
	clientID, _ := c.Get("clientId")
	filingName := c.Query("filing_name")

	if filingName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "filing_name is required"})
		return
	}

	// Build path: root/<clientId>/<filingName>
	baseDir := filepath.Join(root, fmt.Sprintf("%v", clientID), sanitize(filingName))

	// Confirm folder exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"message": "filing not found"})
		return
	}

	// Create temp zip file
	tmpZip, err := os.CreateTemp("", sanitize(filingName)+"_*.zip")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "cannot create zip"})
		return
	}
	tmpZipPath := tmpZip.Name()
	tmpZip.Close()
	defer os.Remove(tmpZipPath)

	// Zip the directory
	if err := zipDirectory(baseDir, tmpZipPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to zip directory"})
		return
	}

	// Download response
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", "attachment; filename="+sanitize(filingName)+".zip")
	c.File(tmpZipPath)
}


func zipDirectory(sourceDir, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if relPath != "." {
				_, err = archive.Create(relPath + "/") 
			}
			return err
		}

		zipEntry, err := archive.Create(relPath)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipEntry, fsFile)
		return err
	})
}
