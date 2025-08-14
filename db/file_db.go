package db

import "form_pf_backend/models"

// GetAllFilings returns a pointer-slice of every row in filing_tracker.
func GetAllFilings() (*[]models.FilingTracker, error) {
	var filings []models.FilingTracker
	err := sqlDb.Model(&models.FilingTracker{}).
		Find(&filings).Error
	return &filings, err
}
