// modules/zipwalk.go
package modules

import (
	"archive/zip"
	"fmt"
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
