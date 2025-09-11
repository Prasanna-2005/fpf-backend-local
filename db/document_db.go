package db

import (
	"form_pf_backend/models"
	"strconv"
	"time"
)

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

func CreateTracker(req *models.SubmitReq, cid string) (int, error) {
	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	uid, err := strconv.Atoi(req.UserID)
	if err != nil {
		return 0, err
	}
	row := models.FilingTracker{
		ClientCode:  cid,
		FilingName:  req.FilingName,
		UploadCount: 1,
		IsSubmitted: true,
		IsDeleted:   false,
		CreatedAt:   ts,
		CreatedBy:   uid,
	}
	if err := sqlDb.Create(&row).Error; err != nil {
		return 0, err
	}
	return int(row.Id), nil
}

func CreateMapper(req *models.SubmitReq, filingId int) ([]models.FileMapper, error) {
	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	uid, err := strconv.Atoi(req.UserID)
	if err != nil {
		return nil, err
	}
	identities, err := GetAllFileMapId()
	if err != nil {
		return nil, err
	}

	// build a quick look-up:  shortCode -> ID
	idByShort := make(map[string]int, len(identities))
	for _, id := range identities {
		idByShort[id.ShortCode] = id.ID
	}

	mapper := make([]models.FileMapper, 0, len(req.FileMappings))
	for _, rec := range req.FileMappings {
		fileMapID := idByShort[rec.Shortcode] // 0 if not found
		mapper = append(mapper, models.FileMapper{
			FileName:      rec.Filename,
			FilingPath:    rec.Filepath,
			FundShortCode: rec.Shortcode,
			FilingID:      filingId,
			FileMapID:     fileMapID,
			QuarterNo:     rec.Quarter,
			IsUpdated:     false,
			CreatedBy:     uid, // string is fine per your ORM
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
