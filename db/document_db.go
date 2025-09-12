package db

import (
	"form_pf_backend/models"
	"strconv"
	"time"

	"gorm.io/gorm"
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
	uid, err := strconv.Atoi(req.UserID)
	if err != nil {
		return err
	}
	tracker.UploadCount += 1
	tracker.UpdatedAt = ts
	tracker.UpdatedBy = uid
	return sqlDb.Save(tracker).Error
}

// UpdateMapper updates every mapper row that belongs to the given filingId:
func UpdateMapper(filingId int, req *models.SubmitReq) error {
	// build a lookup => shortcode : new file data
	newData := make(map[string]struct{ fileName, filePath string })
	for _, m := range req.FileMappings {
		newData[m.Shortcode] = struct{ fileName, filePath string }{m.Filename, m.Filepath}
	}

	// fetch current rows for this filing
	var rows []models.FileMapper
	if err := db.Where("filingId = ?", filingId).Find(&rows).Error; err != nil {
		return err
	}

	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	// update in memory and push back
	for i := range rows {
		if nd, ok := newData[rows[i].FundShortCode]; ok {
			rows[i].FileName = nd.fileName
			rows[i].FilingPath = nd.filePath
			rows[i].IsUpdated = true
			rows[i].UpdatedAt = ts
			rows[i].UpdatedBy = req.UserID
		}
	}

	// 4. persist (Save updates non-zero fields)
	return sqlDb.Session(&gorm.Session{FullSaveAssociations: false}).Save(&rows).Error
}
