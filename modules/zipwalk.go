// modules/zipwalk.go
package modules

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractAndValidateZip extracts all contents of a ZIP file to a dedicated target directory
func ExtractAndValidateZip(zipPath, targetDir string) error {
	logger.Log("DEBUG", fmt.Sprintf("[ExtractAndValidateZip] Starting extraction of %s to %s", zipPath, targetDir))

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		logger.Log("ERROR", fmt.Sprintf("[ExtractAndValidateZip] Failed to open zip %s: %v", zipPath, err))
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	logger.Log("DEBUG", fmt.Sprintf("[ExtractAndValidateZip] Found %d entries in %s", len(r.File), zipPath))

	// Create target directory for this ZIP's contents
	if err := EnsureVolumeDir(targetDir); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// First pass: validate all files before extracting anything
	if err := validateZipContents(&r.Reader); err != nil {
		logger.Log("ERROR", fmt.Sprintf("[ExtractAndValidateZip] Validation failed for %s: %v", zipPath, err))
		return err
	}

	// Second pass: extract all files
	extractedFiles := make([]string, 0, len(r.File))
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			if err := createDirectory(f, targetDir); err != nil {
				cleanup(extractedFiles)
				return err
			}
			continue
		}

		extractedPath, err := extractFile(f, targetDir)
		if err != nil {
			cleanup(extractedFiles)
			return err
		}

		if extractedPath != "" {
			extractedFiles = append(extractedFiles, extractedPath)
		}
	}

	// Process any nested ZIP files that were extracted
	if err := processNestedZips(extractedFiles, targetDir); err != nil {
		cleanup(extractedFiles)
		return err
	}

	logger.Log("INFO", fmt.Sprintf("[ExtractAndValidateZip] Successfully extracted %d files from %s", len(extractedFiles), zipPath))
	return nil
}

// validateZipContents performs a complete validation of all files in the ZIP
func validateZipContents(r *zip.Reader) error {
	validators := map[string]func(*zip.File) error{
		".zip":  validateNestedZip,
		".xls":  func(*zip.File) error { return nil },
		".xlsx": func(*zip.File) error { return nil },
	}

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if err := validatePath(f.Name); err != nil {
			return err
		}

		ext := strings.ToLower(filepath.Ext(filepath.Clean(f.Name)))
		logger.Log("DEBUG", fmt.Sprintf("[validateZipContents] Validating file: %s (ext: %s)", f.Name, ext))

		validator, allowed := validators[ext]
		if !allowed {
			logger.Log("WARNING", fmt.Sprintf("[validateZipContents] Invalid file extension: %s", f.Name))
			return errInvalidFile
		}

		if err := validator(f); err != nil {
			return err
		}
	}
	return nil
}

// validateNestedZip validates the contents of a nested ZIP file without extracting it
func validateNestedZip(f *zip.File) error {
	logger.Log("DEBUG", fmt.Sprintf("[validateNestedZip] Validating nested zip: %s", f.Name))

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("failed to open nested zip %s: %w", f.Name, err)
	}
	defer rc.Close()

	zipData, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read nested zip %s: %w", f.Name, err)
	}

	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return fmt.Errorf("failed to create zip reader for %s: %w", f.Name, err)
	}

	return validateZipContents(zipReader)
}

// validatePath checks for path traversal attacks
func validatePath(path string) error {
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		logger.Log("WARNING", fmt.Sprintf("[validatePath] Suspicious path detected: %s", path))
		return errInvalidFile
	}
	return nil
}

// createDirectory creates a directory from a ZIP entry
func createDirectory(f *zip.File, targetDir string) error {
	if err := validatePath(f.Name); err != nil {
		return err
	}

	dirPath := filepath.Join(targetDir, f.Name)
	logger.Log("DEBUG", fmt.Sprintf("[createDirectory] Creating directory: %s", dirPath))
	return os.MkdirAll(dirPath, f.FileInfo().Mode())
}

// extractFile extracts a single file from the ZIP archive
func extractFile(f *zip.File, targetDir string) (string, error) {
	if err := validatePath(f.Name); err != nil {
		return "", err
	}

	targetPath := filepath.Join(targetDir, f.Name)

	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create parent directories for %s: %w", targetPath, err)
	}

	logger.Log("DEBUG", fmt.Sprintf("[extractFile] Extracting: %s → %s", f.Name, targetPath))

	// Open source file in ZIP
	rc, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file %s in zip: %w", f.Name, err)
	}
	defer rc.Close()

	// Create destination file
	outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.FileInfo().Mode())
	if err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer outFile.Close()

	// Copy file contents
	bytesWritten, err := io.Copy(outFile, rc)
	if err != nil {
		return "", fmt.Errorf("failed to extract file %s: %w", f.Name, err)
	}

	logger.Log("DEBUG", fmt.Sprintf("[extractFile] Successfully extracted %d bytes to %s", bytesWritten, targetPath))
	return targetPath, nil
}

// processNestedZips finds and processes any nested ZIP files that were extracted
func processNestedZips(extractedFiles []string, parentDir string) error {
	for _, filePath := range extractedFiles {
		if strings.EqualFold(filepath.Ext(filePath), ".zip") {
			logger.Log("INFO", fmt.Sprintf("[processNestedZips] Processing nested zip: %s", filePath))

			// Create subdirectory for nested ZIP (key change)
			zipBaseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			nestedDir := filepath.Join(filepath.Dir(filePath), zipBaseName)

			// Recursively extract the nested ZIP to its own directory
			if err := ExtractAndValidateZip(filePath, nestedDir); err != nil {
				logger.Log("ERROR", fmt.Sprintf("[processNestedZips] Failed to process nested zip %s: %v", filePath, err))
				return err
			}
		}
	}
	return nil
}

// cleanup removes all files that were created during extraction (used on error)
func cleanup(filePaths []string) {
	logger.Log("WARNING", "[cleanup] Starting cleanup of extracted files due to error")
	for _, path := range filePaths {
		if err := os.Remove(path); err != nil {
			logger.Log("WARNING", fmt.Sprintf("[cleanup] Failed to remove file %s: %v", path, err))
		} else {
			logger.Log("DEBUG", fmt.Sprintf("[cleanup] Removed file: %s", path))
		}
	}
}
