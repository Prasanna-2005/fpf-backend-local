package db

import (
	"fmt"
	"form_pf_backend/models"
	"path/filepath"
	"sort"
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

// StoredFileVersion is the persisted state used to anchor append-only file
// versions. Versions are assigned once from stored state, never recomputed
// from directory order.
type StoredFileVersion struct {
	ID            string
	FileName      string
	FundShortCode string
	Quarter       string
	Version       int
	IsCurrent     bool
	VolBase       string // uuid-prefixed basename of the stored path
}

func GetStoredFileVersions(filingId string) ([]StoredFileVersion, error) {
	var rows []models.FileMapper
	if err := sqlDb.Where("filingId = ?", filingId).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]StoredFileVersion, 0, len(rows))
	for _, r := range rows {
		out = append(out, StoredFileVersion{
			ID:            r.ID,
			FileName:      r.FileName,
			FundShortCode: r.FundShortCode,
			Quarter:       r.Quarter,
			Version:       r.Version,
			IsCurrent:     r.IsCurrent,
			VolBase:       filepath.Base(r.FilingPath),
		})
	}
	return out, nil
}

// AnchoredFile is the version decision for one incoming mapping.
type AnchoredFile struct {
	Index       int    // index into the input slice (Version is set in place)
	ShortCode   string // canonical group parts, for the isCurrent flip
	QuarterCode string
	Version     int
	Stored      bool // volume copy already persisted: keep version, no new row
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

// AnchorFileVersions reduces mappings to the latest file per slot, then
// versions that file from stored state. A volume path already stored for the
// same group keeps its version. Anything else is max(group version)+1.
func AnchorFileVersions(filingId string, mapped []models.MappedResp) ([]models.MappedResp, []AnchoredFile, error) {
	latest := LatestPerGroup(mapped)
	stored, err := GetStoredFileVersions(filingId)
	if err != nil {
		return nil, nil, err
	}
	byVol := make(map[string]StoredFileVersion, len(stored))
	maxVer := make(map[string]int)
	for _, s := range stored {
		if _, ok := byVol[s.VolBase]; !ok {
			byVol[s.VolBase] = s
		}
		key := s.FundShortCode + "|" + s.Quarter
		if s.Version > maxVer[key] {
			maxVer[key] = s.Version
		}
	}

	out := make([]AnchoredFile, 0, len(latest))
	for i := range latest {
		m := &latest[i]
		shortCode, quarterCode := splitGroupKey(FileGroupKey(m.ShortCode, m.FileType, m.GroupId))
		key := shortCode + "|" + quarterCode
		vol := filepath.Base(m.Path)
		if vol == "" || vol == "." {
			vol = m.FileName
		}
		if s, ok := byVol[vol]; ok && s.FundShortCode == shortCode && s.Quarter == quarterCode {
			m.Version = s.Version
			out = append(out, AnchoredFile{Index: i, ShortCode: shortCode, QuarterCode: quarterCode, Version: s.Version, Stored: true})
			continue
		}
		maxVer[key]++
		m.Version = maxVer[key]
		out = append(out, AnchoredFile{Index: i, ShortCode: shortCode, QuarterCode: quarterCode, Version: m.Version, Stored: false})
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
// Every mapping whose volume file is not yet stored gets a new row at
// maxGroupVersion+1 with isCurrent=true, and the superseded current rows in
// the same (shortCode, quarter) group are flipped to isCurrent=false, all in
// one transaction. Resubmitting an unchanged mapping inserts nothing.
func UpdateMapper(filingId string, req *models.SubmitReq) (time.Time, error) {
	ts, _ := time.Parse(time.RFC3339, req.Timestamp)

	latest, anchored, err := AnchorFileVersions(filingId, req.FileMappings)
	if err != nil {
		return ts, err
	}
	req.FileMappings = latest
	var fresh []AnchoredFile
	for _, a := range anchored {
		if !a.Stored {
			fresh = append(fresh, a)
		}
	}
	if len(fresh) == 0 {
		return ts, nil
	}

	identities, err := GetAllFileMapId()
	if err != nil {
		return ts, err
	}
	idByShort := make(map[string]string, len(identities))
	for _, id := range identities {
		idByShort[id.ShortCode] = id.ID
	}

	return ts, sqlDb.Transaction(func(tx *gorm.DB) error {
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
				Version:       a.Version,
				IsCurrent:     true,
				IsUpdated:     true,
				CreatedAt:     ts,
				CreatedBy:     req.UserID,
			})
		}
		return tx.CreateInBatches(rows, 100).Error
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
