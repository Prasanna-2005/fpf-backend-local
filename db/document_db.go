package db

import (
	"database/sql"
	"fmt"
	"form_pf_backend/models"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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
	return GetDB().Omit("Version").CreateInBatches(rows, 100).Error
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
			IsCurrent:     true,
			IsUpdated:     false,
			CreatedBy:     req.UserID, // string is fine per your ORM
			CreatedAt:     ts,
		})
	}
	return mapper, nil
}

// FileGroupKey returns the canonical identity of a file slot:
// shortCode + stored quarter code. It is the same key whether it is built
// from a mapper response (display file type + group id) or from a stored
// row (fundShortCode + quarterNo), so both sides always agree.
func FileGroupKey(shortCode, fileTypeDisplay, groupId string) string {
	code := FileTypeConvRev[fileTypeDisplay]
	if code == "" {
		if i := strings.LastIndex(groupId, "-"); i >= 0 && strings.HasPrefix(groupId, shortCode+"-") {
			code = groupId[i+1:]
		} else {
			code = fileTypeDisplay
		}
	}
	return shortCode + "|" + code
}

func splitGroupKey(key string) (string, string) {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		return key, ""
	}
	return parts[0], parts[1]
}

// StoredFile is a persisted file_mapper row, used to tell whether an
// incoming volume path is already stored for a slot.
type StoredFile struct {
	ID            string
	FileName      string
	FundShortCode string
	Quarter       string
	IsCurrent     bool
	VolBase       string // uuid-prefixed basename of the stored path
}

func GetStoredFiles(tx *gorm.DB, filingId string) ([]StoredFile, error) {
	var rows []models.FileMapper
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("filingId = ?", filingId).
		Find(&rows).Error; err != nil {
		log.Printf("GetStoredFiles: filing %s: %v", filingId, err)
		return nil, err
	}
	out := make([]StoredFile, 0, len(rows))
	for _, r := range rows {
		out = append(out, StoredFile{
			ID:            r.ID,
			FileName:      r.FileName,
			FundShortCode: r.FundShortCode,
			Quarter:       r.Quarter,
			IsCurrent:     r.IsCurrent,
			VolBase:       filepath.Base(r.FilingPath),
		})
	}
	return out, nil
}

// AnchoredFile is the stored-or-new decision for one incoming mapping.
type AnchoredFile struct {
	Index       int
	ShortCode   string // canonical group parts, for the isCurrent flip
	QuarterCode string
	Stored      bool // volume copy already persisted: no new row
}

// LatestPerGroup keeps one mapping per slot: the newest upload in that
// short-code, file-type, and period. Callers can return this list directly,
// so the UI does not choose among copies.
func LatestPerGroup(mapped []models.MappedResp) []models.MappedResp {
	order := make([]int, 0, len(mapped))
	for i := range mapped {
		order = append(order, i)
	}
	sort.SliceStable(order, func(a, b int) bool {
		return mapped[order[a]].Timestamp.Before(mapped[order[b]].Timestamp)
	})

	chosen := make(map[string]models.MappedResp, len(mapped))
	keyOrder := make([]string, 0, len(mapped))
	for _, i := range order {
		m := mapped[i]
		key := slotKey(m)
		if _, ok := chosen[key]; !ok {
			keyOrder = append(keyOrder, key)
		}
		chosen[key] = m
	}
	out := make([]models.MappedResp, 0, len(keyOrder))
	for _, key := range keyOrder {
		out = append(out, chosen[key])
	}
	return out
}

func slotKey(m models.MappedResp) string {
	return FileGroupKey(m.ShortCode, m.FileType, m.GroupId) + "|" + strings.ToLower(m.Period)
}

// AnchorIncomingFiles reduces mappings to the latest file per slot, then
// marks a volume path already stored for the same group so it is not
// inserted again. New files are identified only by isCurrent.
func AnchorIncomingFiles(tx *gorm.DB, filingId string, mapped []models.MappedResp) ([]models.MappedResp, []AnchoredFile, error) {
	latest := LatestPerGroup(mapped)
	stored, err := GetStoredFiles(tx, filingId)
	if err != nil {
		return nil, nil, err
	}
	byVol := make(map[string]StoredFile, len(stored))
	for _, s := range stored {
		key := s.FundShortCode + "|" + s.Quarter + "|" + s.VolBase
		if _, ok := byVol[key]; !ok {
			byVol[key] = s
		}
	}

	out := make([]AnchoredFile, 0, len(latest))
	for i := range latest {
		m := &latest[i]
		shortCode, quarterCode := splitGroupKey(FileGroupKey(m.ShortCode, m.FileType, m.GroupId))
		vol := filepath.Base(m.Path)
		if vol == "" || vol == "." {
			vol = m.FileName
		}
		key := shortCode + "|" + quarterCode + "|" + vol
		if s, ok := byVol[key]; ok && s.FundShortCode == shortCode && s.Quarter == quarterCode {
			out = append(out, AnchoredFile{Index: i, ShortCode: shortCode, QuarterCode: quarterCode, Stored: true})
			continue
		}
		out = append(out, AnchoredFile{Index: i, ShortCode: shortCode, QuarterCode: quarterCode, Stored: false})
	}
	return latest, out, nil
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

// UpdateMapper persists an upload-more submit for an existing filing.
// Every mapping whose volume file is not yet stored gets a new row with
// isCurrent=true, and the superseded current rows in the same
// (shortCode, quarter) group are flipped to isCurrent=false, all in one
// transaction. Resubmitting an unchanged mapping inserts nothing.
func UpdateMapper(filingId string, req *models.SubmitReq) (time.Time, error) {
	ts, err := time.Parse(time.RFC3339, req.Timestamp)
	if err != nil {
		log.Printf("UpdateMapper: invalid timestamp %q for filing %s: %v", req.Timestamp, filingId, err)
	}

	identities, err := GetAllFileMapId()
	if err != nil {
		log.Printf("UpdateMapper: load file map ids for filing %s: %v", filingId, err)
		return ts, err
	}
	idByShort := make(map[string]string, len(identities))
	for _, id := range identities {
		idByShort[id.ShortCode] = id.ID
	}

	return ts, sqlDb.Transaction(func(tx *gorm.DB) error {
		// Lock the filing row first. SELECT FOR UPDATE on file_mapper locks
		// nothing when the filing has no rows yet, so two first submits could
		// both insert a current row for the same slot.
		var tracker models.FilingTracker
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("id").
			Where("id = ?", filingId).
			Take(&tracker).Error; err != nil {
			log.Printf("UpdateMapper: lock filing %s: %v", filingId, err)
			return err
		}

		latest, anchored, err := AnchorIncomingFiles(tx, filingId, req.FileMappings)
		if err != nil {
			return err
		}
		req.FileMappings = latest
		var fresh []AnchoredFile
		for _, a := range anchored {
			if !a.Stored {
				fresh = append(fresh, a)
			}
		}
		if len(fresh) == 0 {
			return nil
		}

		seenGroup := make(map[string]bool, len(fresh))
		for _, a := range fresh {
			key := a.ShortCode + "|" + a.QuarterCode
			if seenGroup[key] {
				continue
			}
			seenGroup[key] = true
			if err := tx.Model(&models.FileMapper{}).
				Where("filingId = ? AND fundShortCode = ? AND quarterNo = ? AND isCurrent = ?", filingId, a.ShortCode, a.QuarterCode, true).
				Update("isCurrent", false).Error; err != nil {
				log.Printf("UpdateMapper: clear current file for filing %s group %s: %v", filingId, key, err)
				return err
			}
		}
		rows := make([]models.FileMapper, 0, len(fresh))
		for _, a := range fresh {
			m := req.FileMappings[a.Index]
			rows = append(rows, models.FileMapper{
				FileName:      m.FileName,
				FilingPath:    m.Path,
				FundShortCode: a.ShortCode,
				FilingID:      filingId,
				FileMapID:     idByShort[m.ShortCode],
				Quarter:       a.QuarterCode,
				Period:        m.Period,
				IsCurrent:     true,
				IsUpdated:     true,
				CreatedAt:     ts,
				CreatedBy:     req.UserID,
			})
		}
		if err := tx.Omit("Version").CreateInBatches(rows, 100).Error; err != nil {
			log.Printf("UpdateMapper: insert files for filing %s: %v", filingId, err)
			return err
		}
		return nil
	})
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

// PullFilingMapper returns the files that were in the mapper when a version
// was extracted, including files no question opened. file_mapper has no
// version column, so the cutoff is the earliest non-edit createdAt of that
// form_data_legacy version. version <= 0 uses the filing's current version.
// A version with no extraction rows falls back to the latest current copy.
func PullFilingMapper(filingId string, series string, version int) ([]models.MappedResp, error) {
	fundMap, err := PullFundName(series)
	if err != nil {
		log.Printf("PullFilingMapper: fund name map for filing %s series %s: %v", filingId, series, err)
		return nil, fmt.Errorf("failed to get fund name map: %w", err)
	}

	if version <= 0 {
		version, err = filingHeadVersion(filingId)
		if err != nil {
			log.Printf("PullFilingMapper: head version for filing %s: %v", filingId, err)
			return nil, err
		}
	}

	extractedAt, found, err := versionExtractedAt(filingId, version)
	if err != nil {
		return nil, err
	}
	if !found {
		log.Printf("PullFilingMapper: filing %s version %d has no extraction time; using current copies", filingId, version)
		return currentMappedFiles(filingId, fundMap)
	}

	var rows []models.FileMapper
	result := sqlDb.Select("fileName", "filingPath", "fundShortCode", "quarterNo", "period", "createdAt").
		Where("filingId = ?", filingId).
		Order("createdAt ASC").
		Find(&rows)
	if result.Error != nil {
		log.Printf("PullFilingMapper: load files for filing %s: %v", filingId, result.Error)
		return nil, result.Error
	}

	return withFundNames(filesPresentAt(rows, extractedAt), fundMap), nil
}

func filingHeadVersion(filingId string) (int, error) {
	var version int
	err := sqlDb.Model(&models.FilingTracker{}).
		Where("id = ?", filingId).
		Pluck("currentVersion", &version).Error
	return version, err
}

// versionExtractedAt is the earliest non-edit write for a form version.
// Edits in the same version are later and must not move the cutoff.
func versionExtractedAt(filingId string, version int) (time.Time, bool, error) {
	var row struct {
		ExtractedAt sql.NullTime `gorm:"column:extractedAt"`
	}
	err := sqlDb.Model(&models.FormDataLegacy{}).
		Select("MIN(createdAt) AS extractedAt").
		Where("filingId = ? AND version = ? AND isEdit = ?", filingId, version, false).
		Scan(&row).Error
	if err != nil {
		log.Printf("PullFilingMapper: extraction time for filing %s version %d: %v", filingId, version, err)
		return time.Time{}, false, err
	}
	if !row.ExtractedAt.Valid {
		return time.Time{}, false, nil
	}
	return row.ExtractedAt.Time, true, nil
}

func currentMappedFiles(filingId string, fundMap map[string]string) ([]models.MappedResp, error) {
	var rows []models.FileMapper
	result := sqlDb.Select("fileName", "filingPath", "fundShortCode", "quarterNo", "period", "createdAt").
		Where("filingId = ? AND isCurrent = ?", filingId, true).
		Order("createdAt ASC").
		Find(&rows)
	if result.Error != nil {
		log.Printf("PullFilingMapper: load current files for filing %s: %v", filingId, result.Error)
		return nil, result.Error
	}

	return withFundNames(LatestPerGroup(toMappedResp(rows)), fundMap), nil
}

// filesPresentAt keeps one file per slot from the copies submitted at or
// before extraction. A later replacement of the same slot stays on a later version.
func filesPresentAt(rows []models.FileMapper, extractedAt time.Time) []models.MappedResp {
	present := make([]models.FileMapper, 0, len(rows))
	for _, row := range rows {
		if row.CreatedAt.After(extractedAt) {
			continue
		}
		present = append(present, row)
	}
	return LatestPerGroup(toMappedResp(present))
}

func toMappedResp(rows []models.FileMapper) []models.MappedResp {
	out := make([]models.MappedResp, 0, len(rows))
	for _, row := range rows {
		out = append(out, models.MappedResp{
			FileName:  row.FileName,
			Path:      row.FilingPath,
			ShortCode: row.FundShortCode,
			FileType:  row.Quarter,
			Period:    row.Period,
			Timestamp: row.CreatedAt,
		})
	}
	return out
}

func withFundNames(mapped []models.MappedResp, fundMap map[string]string) []models.MappedResp {
	resp := make([]models.MappedResp, 0, len(mapped))
	for _, m := range mapped {
		resp = append(resp, models.MappedResp{
			FileName:  m.FileName,
			Path:      m.Path,
			ShortCode: m.ShortCode,
			FileType:  m.FileType,
			FundName:  fundMap[strings.ToLower(m.ShortCode)],
			Period:    m.Period,
		})
	}
	return resp
}

func GetCurrentMappers(filingId string) ([]models.FileMapper, error) {
	var rows []models.FileMapper
	err := sqlDb.Where("filingId = ? AND isCurrent = ?", filingId, true).Find(&rows).Error
	return rows, err
}
