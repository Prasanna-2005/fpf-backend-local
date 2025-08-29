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
