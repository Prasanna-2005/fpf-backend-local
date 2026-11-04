package db

import (
	"fmt"
	"form_pf_backend/models"
	"time"
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

// FilingListRow is one filings-list entry, with the user names already resolved.
type FilingListRow struct {
	ID           string    `gorm:"column:id"`
	FilingName   string    `gorm:"column:filingName"`
	IsSubmitted  bool      `gorm:"column:isSubmitted"`
	IsDeleted    bool      `gorm:"column:isDeleted"`
	CreatedAt    time.Time `gorm:"column:createdAt"`
	UpdatedAt    time.Time `gorm:"column:updatedAt"`
	FilingStatus string    `gorm:"column:filingStatus"`
	PreparedBy   string    `gorm:"column:preparedBy"`
	ModifiedBy   string    `gorm:"column:modifiedBy"`
}

// ListFilingsForClient returns that client's filings and the names of the users
// who created and last updated them, in one query.
func ListFilingsForClient(clientCode string) ([]FilingListRow, error) {
	tracker := models.FilingTracker{}.TableName()
	users := models.User{}.TableName()
	var rows []FilingListRow
	err := sqlDb.Table(tracker).
		Select(tracker+".id, "+tracker+".filingName, "+tracker+".isSubmitted, "+tracker+".isDeleted, "+tracker+".createdAt, "+tracker+".updatedAt, "+tracker+".filingStatus, prepared.userName AS preparedBy, modifier.userName AS modifiedBy").
		Joins("LEFT JOIN "+users+" AS prepared ON prepared.id = "+tracker+".createdBy").
		Joins("LEFT JOIN "+users+" AS modifier ON modifier.id = "+tracker+".updatedBy").
		Where(tracker+".clientCode = ?", clientCode).
		Scan(&rows).Error
	return rows, err
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
