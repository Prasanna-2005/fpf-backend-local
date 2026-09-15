package db

import (
	"fmt"
	"form_pf_backend/models"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var FileTypeConvRev = map[string]string{
	"Capital Register":     "capital_register",
	"Trial Balance":        "nav_tb",
	"Performance report":   "nav_report",
	"Unfunded_Commitments": "Unfunded_Commitments",
	"NA":                   "NA",
	"Q3":                   "3",
	"Q2":                   "2",
	"Q1":                   "1",
}

// GetAllFegFileIdentifications -> models.FegFileIdentification
func GetAllFegFileIdentifications() ([]models.FegFileIdentification, error) {
	var list []models.FegFileIdentification
	err := GetDB().Find(&list).Error
	return list, err
}

// GetFegFileIdentificationByFund -> models.FegFileIdentification   //not needed
func GetFegFileIdentificationByFund(fundName string, quarter int) (*models.FegFileIdentification, error) {
	var h models.FegFileIdentification
	err := GetDB().Where("fund_name = ? AND quarter = ?", fundName, quarter).First(&h).Error
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// CreateFegFileIdentification -> insert header
func CreateFegFileIdentification(h *models.FegFileIdentification) error {
	return GetDB().Create(h).Error
}

// BatchSaveFileMappings -> insert children
func BatchSaveFileMappings(rows []models.FileMapper) error {
	return GetDB().CreateInBatches(rows, 100).Error
}

func CreateTracker(req *models.SubmitReq, cid string) (string, error) {
	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	defaultStatus := "In progress"

	row := models.FilingTracker{
		Id:           uuid.New().String(),
		ClientCode:   cid,
		Series:       req.Series,
		FilingName:   req.FilingName,
		UploadCount:  1,
		IsSubmitted:  true,
		IsDeleted:    false,
		CreatedAt:    ts,
		CreatedBy:    req.UserID,
		FilingStatus: defaultStatus,
	}
	if err := sqlDb.Create(&row).Error; err != nil { // normal call
		return "", err
	}
	return row.Id, nil
}

func CreateMapper(req *models.SubmitReq, filingId string) ([]models.FileMapper, error) {
	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	identities, err := GetAllFileMapId()
	if err != nil {
		return nil, err
	}

	// build a quick look-up:  shortCode -> ID
	idByShort := make(map[string]string, len(identities))
	for _, id := range identities {
		idByShort[id.ShortCode] = id.ID
	}

	mapper := make([]models.FileMapper, 0, len(req.FileMappings))
	for _, rec := range req.FileMappings {
		fileMapID := idByShort[rec.ShortCode] // 0 if not found
		mapper = append(mapper, models.FileMapper{
			FileName:      rec.FileName,
			FilingPath:    rec.Path,
			FundShortCode: rec.ShortCode,
			FilingID:      filingId,
			FileMapID:     fileMapID,
			Quarter:       FileTypeConvRev[rec.FileType],
			Period:        rec.Period,
			Version:       1,    // version starts with 1
			IsCurrent:     true, // active version
			IsUpdated:     false,
			CreatedBy:     req.UserID, // string is fine per your ORM
			CreatedAt:     ts,
		})
	}
	return mapper, nil
}

func GetAllFileMapId() ([]models.FegFileIdentification, error) {
	var list []models.FegFileIdentification
	err := sqlDb.Find(&list).Error
	return list, err
}

func GetFilingTrackerByName(filingName string) (*models.FilingTracker, error) {
	var ft models.FilingTracker
	err := sqlDb.Where("filingName = ?", filingName).First(&ft).Error
	if err != nil {
		return nil, err
	}
	return &ft, nil
}

func UpdateTracker(tracker *models.FilingTracker, req *models.SubmitReq) error {
	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	tracker.UploadCount += 1
	tracker.UpdatedAt = ts
	tracker.UpdatedBy = req.UserID
	return sqlDb.Save(tracker).Error
}

// tracker status update
func UpdateTrackerStatus(tracker *models.FilingTracker, userId string, status string, ts time.Time) error {
	// ts, _ := time.Parse(time.RFC3339, req.Timestamp)
	// uid, err := strconv.Atoi(userID)
	// if err != nil {
	// 	return err
	// }
	tracker.FilingStatus = status
	tracker.UpdatedAt = ts
	tracker.UpdatedBy = userId

	if status == "Archived" {
		tracker.IsDeleted = true
	}
	return sqlDb.Save(tracker).Error
}

// UpdateMapper updates every mapper row that belongs to the given filingId:
func UpdateMapper(filingId string, req *models.SubmitReq) (time.Time, error) {
	// 1. build lookup: short-code -> new file data
	newData := make(map[string]struct{ fileName, filePath string })
	for _, m := range req.FileMappings {
		newData[fmt.Sprintf("%s-%d", m.GroupId, m.Version)] = struct{ fileName, filePath string }{m.FileName, m.Path}
	}

	ts, _ := time.Parse(time.RFC3339, req.Timestamp)
	// 2. fetch current rows for this filing
	var rows []models.FileMapper
	if err := sqlDb.Where("filingId = ?", filingId).Find(&rows).Error; err != nil {
		return ts, err
	}

	// 3. update existing rows (in memory)
	existingSC := make(map[string]bool, len(rows))
	for _, m := range rows {
		existingSC[fmt.Sprintf("%s-%s-%d", m.FundShortCode, m.Quarter, m.Version)] = true
	}
	// 4. persist updates
	if err := sqlDb.Session(&gorm.Session{FullSaveAssociations: false}).Save(&rows).Error; err != nil {
		return ts, err
	}

	// 5. insert brand-new files (short-codes not yet in DB)
	var toInsert []models.FileMapper
	for _, m := range req.FileMappings {
		if !existingSC[fmt.Sprintf("%s-%d", m.GroupId, m.Version)] {
			toInsert = append(toInsert, models.FileMapper{
				FileName:      m.FileName,
				FilingPath:    m.Path,
				FundShortCode: m.ShortCode,
				FilingID:      filingId,
				Quarter:       FileTypeConvRev[m.FileType],
				Period:        m.Period,
				Version:       m.Version,
				IsUpdated:     true,
				CreatedAt:     ts,
				CreatedBy:     req.UserID,
			})
		}
	}
	if len(toInsert) > 0 {
		if err := sqlDb.Create(&toInsert).Error; err != nil {
			return ts, err
		}
	}

	return ts, nil
}

// PullFundName returns a map of shortCode -> fundName for a given series
func PullFundName(series string) (map[string]string, error) {
	type FundMapping struct {
		ShortCode string `gorm:"column:shortcode"`
		FundName  string `gorm:"column:fundname"`
	}

	var mappings []FundMapping

	result := sqlDb.Table("formpf_stage.feg_file_identification").
		Select("DISTINCT shortcode", "fundname").
		Where("series = ?", series).
		Find(&mappings)

	if result.Error != nil {
		return nil, result.Error
	}

	// Build map: shortCode -> fundName
	fundMap := make(map[string]string)
	for _, m := range mappings {
		fundMap[strings.ToLower(m.ShortCode)] = m.FundName
	}

	return fundMap, nil
}

// PullFilingMapper pulls all file mappers for a filing and enriches with fund names
func PullFilingMapper(filingId string, series string) ([]models.MappedResp, error) {
	var rows []models.FileMapper
	var resp []models.MappedResp

	// Get fund name map for the series
	fundMap, err := PullFundName(series)
	if err != nil {
		return resp, fmt.Errorf("failed to get fund name map: %w", err)
	}

	// Select all matching records

	result := sqlDb.Select("fileName", "filingPath", "fundShortCode", "quarterNo").
		Where("filingId = ? AND isCurrent = ?", filingId, true).
		Find(&rows) // Changed from First to Find

	if result.Error != nil {
		return resp, result.Error
	}

	// Map each row to response
	for _, row := range rows {
		// Lookup fundname using  shortcode
		fundName := fundMap[strings.ToLower(row.FundShortCode)]

		mapped := models.MappedResp{
			FileName:  row.FileName,
			Path:      row.FilingPath,
			ShortCode: row.FundShortCode,
			FileType:  row.Quarter,
			FundName:  fundName,
		}
		resp = append(resp, mapped)
	}

	return resp, nil
}

func GetCurrentMappers(filingId string) ([]models.FileMapper, error) {
	var rows []models.FileMapper
	err := sqlDb.Where("filingId = ? AND isCurrent = ?", filingId, true).Find(&rows).Error
	return rows, err
}

func ReplaceMapperVersions(filingId string, items []models.PendingVersion, userId string) error {
	if len(items) == 0 {
		return nil
	}

	oldIDs := make([]string, 0, len(items))
	for _, it := range items {
		oldIDs = append(oldIDs, it.OldMapID)
	}

	var oldRows []models.FileMapper
	if err := sqlDb.Where("id IN ? AND filingId = ?", oldIDs, filingId).Find(&oldRows).Error; err != nil {
		return err
	}
	oldByID := make(map[string]models.FileMapper, len(oldRows))
	for _, r := range oldRows {
		oldByID[r.ID] = r
	}

	ts := time.Now()
	newRows := make([]models.FileMapper, 0, len(items))
	for _, it := range items {
		old, ok := oldByID[it.OldMapID]
		if !ok {
			continue // stale/mismatched OldMapID — skip rather than fail the whole batch
		}
		newRows = append(newRows, models.FileMapper{
			FileName:      it.Filename,
			FilingPath:    "", // filled in by caller once the staged path is finalized
			FundShortCode: old.FundShortCode,
			FilingID:      filingId,
			FileMapID:     old.FileMapID,
			Quarter:       old.Quarter,
			Period:        old.Period,
			Version:       old.Version + 1,
			IsCurrent:     true,
			IsUpdated:     true,
			CreatedAt:     ts,
			CreatedBy:     userId,
		})
	}
	if len(newRows) == 0 {
		return fmt.Errorf("no matching current mappings found for given items")
	}

	return sqlDb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.FileMapper{}).
			Where("id IN ?", oldIDs).
			Update("isCurrent", false).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(newRows, 100).Error
	})
}
