package db

import (
	"fmt"
	"form_pf_backend/models"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var FileTypeConvRev = map[string]string{
	"Capital Register":   "capital_register",
	"Trial Balance":      "nav_tb",
	"Performance report": "nav_report",
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

	defaultStatus:="In progress"

	row := models.FilingTracker{
		Id:          uuid.New().String(),
		ClientCode:  cid,
		FilingName:  req.FilingName,
		UploadCount: 1,
		IsSubmitted: true,
		IsDeleted:   false,
		CreatedAt:   ts,
		CreatedBy:   req.UserID,
		FilingStatus:  defaultStatus,
	}
	if err := sqlDb.Create(&row).Error; err != nil { // normal call
		return "", err
	}

	fmt.Printf("row ID: %vhi\n", row)
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

//tracker status update
func UpdateTrackerStatus(tracker *models.FilingTracker,userId string, status string,ts time.Time) error {
	// ts, _ := time.Parse(time.RFC3339, req.Timestamp)
	// uid, err := strconv.Atoi(userID)
	// if err != nil {
	// 	return err
	// }
	tracker.FilingStatus = status
	tracker.UpdatedAt = ts
	tracker.UpdatedBy = userId

	if status=="Archived"{
		tracker.IsDeleted=true
	}
	return sqlDb.Save(tracker).Error
}


// UpdateMapper updates every mapper row that belongs to the given filingId:
func UpdateMapper(filingId string, req *models.SubmitReq) error {
	// 1. build lookup: short-code -> new file data
	newData := make(map[string]struct{ fileName, filePath string })
	for _, m := range req.FileMappings {
		newData[m.ShortCode] = struct{ fileName, filePath string }{m.FileName, m.Path}
	}

	// 2. fetch current rows for this filing
	var rows []models.FileMapper
	if err := sqlDb.Where("filingId = ?", filingId).Find(&rows).Error; err != nil {
		return err
	}

	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	// 3. update existing rows (in memory)
	existingSC := make(map[string]bool, len(rows))
	for i := range rows {
		existingSC[rows[i].FundShortCode] = true
		if nd, ok := newData[rows[i].FundShortCode]; ok {
			rows[i].FileName = nd.fileName
			rows[i].FilingPath = nd.filePath
			rows[i].IsUpdated = true
			rows[i].UpdatedAt = ts
			rows[i].UpdatedBy = req.UserID
		}
	}

	// 4. persist updates
	if err := sqlDb.Session(&gorm.Session{FullSaveAssociations: false}).Save(&rows).Error; err != nil {
		return err
	}

	// 5. insert brand-new files (short-codes not yet in DB)
	var toInsert []models.FileMapper
	for _, m := range req.FileMappings {
		if !existingSC[m.ShortCode] {
			toInsert = append(toInsert, models.FileMapper{
				FileName:      m.FileName,
				FilingPath:    m.Path,
				FundShortCode: m.ShortCode,
				FilingID:      filingId,
				Quarter:       m.FileType,
				IsUpdated:     true,
				CreatedAt:     ts,
				CreatedBy:     req.UserID,
			})
		}
	}
	if len(toInsert) > 0 {
		if err := sqlDb.Create(&toInsert).Error; err != nil {
			return err
		}
	}

	return nil
}
