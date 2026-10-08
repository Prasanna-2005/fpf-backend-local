package db

import (
	"fmt"
	"form_pf_backend/models"
)

// GetAllFilings returns a pointer-slice of every row in filing_tracker.
func GetAllFilings() (*[]models.FilingTracker, error) {
	var filings []models.FilingTracker
	// err := dbAlive(sqlDb)
	// if !ok {
	// 	return &filings, ServerError
	// }

	err := sqlDb.Model(&models.FilingTracker{}).
		Find(&filings).Error

	return &filings, err
}

func GetSeries(filingId, clientCode string) (string, error) {
	var filing models.FilingTracker
	// err := dbAlive(sqlDb)
	// if !ok {
	// 	return &filings, ServerError
	// }

	err := sqlDb.Model(&models.FilingTracker{}).
		Where("id = ? AND clientCode=?", filingId, clientCode).
		Find(&filing).Error
	if err != nil {
		return "", err
	}

	return filing.Series, err
}

func GetFilingsByClientCode(clientCode string) (*[]models.FilingTracker, error) {
	var filings []models.FilingTracker

	err := sqlDb.Model(&models.FilingTracker{}).
		Where("ClientCode = ?", clientCode).
		Find(&filings).Error

	return &filings, err
}

// FilingStatus
func GetFilingStatus(filingID string) (*models.FilingStatus, error) {
	var fs models.FilingStatus
	err := sqlDb.Where("id = ?", filingID).First(&fs).Error
	if err != nil {
		return nil, err
	}
	return &fs, nil
}

// FilingTracker
func GetFilingTracker(filingID string) (*models.FilingTracker, error) {
	var ft models.FilingTracker
	err := sqlDb.Where("id = ?", filingID).First(&ft).Error
	if err != nil {
		return nil, err
	}
	return &ft, nil
}

// GetRelatedFilingsBySeries returns other filings that share the same company
// series and client code, excluding the current filing and deleted rows.
func GetRelatedFilingsBySeries(filingID, series, clientCode string) ([]models.FilingTracker, error) {
	var filings []models.FilingTracker
	err := sqlDb.Model(&models.FilingTracker{}).
		Where("series = ? AND id <> ? AND clientCode = ? AND isDeleted = ?", series, filingID, clientCode, false).
		Find(&filings).Error
	return filings, err
}

func GetFilingId(filingName, cid string) (*string, *string, error) {
	var result struct {
		Id     *string
		Series *string
	}
	if filingName == "" {
		return nil, nil, fmt.Errorf("Empty filing name present")
	}
	err := sqlDb.Model(&models.FilingTracker{}).
		Select("id,series").
		Where("filingName = ? and clientCode=?", filingName, cid).
		Scan(&result).Error
	if err != nil {
		return nil, nil, err
	}
	return result.Id, result.Series, nil
}
