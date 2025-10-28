package db

import (
	"errors"
	"fmt"
	"form_pf_backend/models"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	ErrFilingNotFound = errors.New("no filing found for the provided filing_id")
	ErrNothingToDo    = errors.New("no update")
	ServerError       = errors.New("server down")
)

var (
	fundCache     map[string]fundBrief
	onceFundCache sync.Once
)

type fundBrief struct {
	FundID   string
	FundName string
}

func loadFundCache() {
	all, _ := GetAllFunds()
	m := make(map[string]fundBrief, len(all))
	for _, f := range all {
		m[f.FundID] = fundBrief{FundID: f.FundID, FundName: f.FundName}
	}
	fundCache = m
}

type pair struct{ ln, vl string }

// GetFundBrief returns fundID & fundName for a fundSeriesId
func GetFundBrief(seriesID string) (fundID, fundName string, err error) {
	onceFundCache.Do(loadFundCache)
	b, ok := fundCache[seriesID]
	if !ok {
		return "", "", gorm.ErrRecordNotFound
	}
	return b.FundID, b.FundName, nil
}

func FetchFormDataByFiling(filingID uint64) ([]models.FormDatum, error) {
	var rows []models.FormDatum
	err := GetDB().
		Table("form_fields_ms_legacy fld").
		Select(`d.createdAt, d.createdBy, d.updatedAt, d.updatedBy, d.value,
		        d.fundSeriesId,
		        f.fundName, f.fundType,
		        fld.fieldNo, fld.formLineNo, fld.lineText, fld.dataType,
		        fld.isMultipleAdd, fld.isMSQ, fld.isMCQ,
		        fld.isTableData, fld.disclaimerText, fld.xmlTags`).
		Joins("LEFT JOIN form_data_legacy d ON fld.fieldNo = d.fieldNo AND "+
			"fld.formLineNo = d.formLineNo AND "+
			"d.filingId = ? AND d.isCurrentValue = 1", filingID).
		Joins("INNER JOIN feg_funds f ON f.fegFundId = d.fundSeriesId").
		Order(`d.fundSeriesId IS NULL, d.fundSeriesId, fld.fieldNo, fld.formLineNo`). // NULLs last, then by fund, then field/line
		Limit(400).
		Find(&rows).Error
	return rows, err
}

// UpdateLineItemFast – same signature as before, but no updateSingleRecord
func UpdateLineItem(data *models.FieldItem, fid, uid, fundId string) error {
	sqlDb := GetDB()
	if !dbAlive(sqlDb) {
		return ServerError
	}

	// 1. filing exists check (single SELECT, uses PK index)
	var cnt int64
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ?", fid).
		Limit(1).
		Count(&cnt).Error; err != nil || cnt == 0 {
		return ErrFilingNotFound
	}

	// collect every (lineNo, value) pair we need to upsert

	var pairs []pair
	now := time.Now()

	// TABLE branch
	if data.HasTableData && data.TableData != nil {
		rows, err := castTableData(data.TableData)
		if err != nil {
			return err
		}
		cells := explodeTableCells(data.FieldNo, rows)
		for ln, vl := range cells {
			pairs = append(pairs, pair{ln: ln, vl: vl})
		}
	} else {
		// single field
		if data.Value != "" {
			pairs = append(pairs, pair{ln: data.FieldNo, vl: data.Value})
		}
		// FieldData slice
		for _, fd := range data.FieldData {
			pairs = append(pairs, pair{ln: fd.LineItemNo, vl: fd.Value})
		}
	}
	if len(pairs) == 0 {
		return nil // nothing to do
	}

	// 2. one UPDATE to mark old rows non-current
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND isCurrentValue = ? AND formLineNo IN ?",
			fid, fundId, data.FieldNo, true, pairLineNos(pairs)).
		Update("isCurrentValue", false).Error; err != nil {
		return err
	}

	// 3. bulk INSERT new rows
	rowsToInsert := make([]models.FormDataLegacy, len(pairs))
	for i, p := range pairs {
		rowsToInsert[i] = models.FormDataLegacy{
			FilingID:       fid,
			FundSeriesid:   fundId,
			FieldNo:        data.FieldNo,
			FormLineNo:     p.ln,
			Value:          p.vl,
			IsCurrentValue: true,
			CreatedBy:      uid,
			CreatedAt:      now,
			UpdatedBy:      uid,
		}
	}
	// CreateInBatches sends one INSERT with multiple values
	return sqlDb.CreateInBatches(rowsToInsert, 200).Error
}

// helper to extract []string of line numbers for the IN clause
func pairLineNos(pp []pair) []string {
	out := make([]string, len(pp))
	for i, p := range pp {
		out[i] = p.ln
	}
	return out
}

// explodeTableCells
func explodeTableCells(fieldNo string, rows []map[string]interface{}) map[string]string {
	out := make(map[string]string, len(rows)*4)
	for _, row := range rows {
		rk, ok := row["s.name"].(string)
		if !ok || rk == "" {
			continue
		}
		for ck, val := range row {
			if ck == "s.name" || ck == "s.text" {
				continue // meta keys
			}
			formLineNo := fmt.Sprintf("%s_%s_%s", fieldNo, strings.TrimSpace(rk), ck)
			if v, ok := val.(string); ok {
				out[formLineNo] = v
			}
		}
	}
	return out
}

// safe table extractor
func castTableData(raw interface{}) ([]map[string]interface{}, error) {
	slice, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("table_data is not an array")
	}
	out := make([]map[string]interface{}, 0, len(slice))
	for _, v := range slice {
		row, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("table row is not an object")
		}
		out = append(out, row)
	}
	return out, nil
}

func RevertLineItem(data *models.FieldItem, fid, uid, fundId string) error {
	sqlDb := GetDB()
	if !dbAlive(sqlDb) {
		return ServerError
	}

	//filing exists
	var cnt int64
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ?", fid).Count(&cnt).Error; err != nil || cnt == 0 {
		return ErrFilingNotFound
	}

	//  TABLE  (many rows)
	if data.HasTableData && data.TableData != nil {
		rows, err := castTableData(data.TableData)
		if err != nil {
			return err
		}
		cells := explodeTableCells(data.FieldNo, rows)
		for formLineNo := range cells {
			// revert each cell independently
			if err := revertSingleRecord(sqlDb, fid, uid, fundId,
				data.FieldNo, formLineNo); err != nil && !errors.Is(err, ErrNothingToDo) {
				return err
			}
		}
		return nil
	}

	//BOOL / TEXT / OPTIONS  (single rows)
	//outside value
	if data.Value != "" {
		if err := revertSingleRecord(sqlDb, fid, uid, fundId,
			data.FieldNo, data.FieldNo); err != nil && !errors.Is(err, ErrNothingToDo) {
			return err
		}
	}
	//inside field_data lines
	for _, line := range data.FieldData {
		if err := revertSingleRecord(sqlDb, fid, uid, fundId,
			data.FieldNo, line.LineItemNo); err != nil && !errors.Is(err, ErrNothingToDo) {
			return err
		}
	}
	return nil
}

func revertSingleRecord(sqlDb *gorm.DB,
	fid, uid, fundId, fieldNo, formLineNo string) error {

	// 1. Find the current active record
	type recordVersion struct {
		ID        uint      `gorm:"column:id"`
		CreatedAt time.Time `gorm:"column:createdAt"`
	}

	var currentRecord recordVersion
	if err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Select("id, createdAt").
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ? AND isCurrentValue = ?",
			fid, fundId, fieldNo, formLineNo, true).
		First(&currentRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[INFO] No active record found to revert")
			return ErrNothingToDo
		}
		log.Printf("[ERROR] Failed to fetch current record: %v", err)
		return err
	}

	// 2. Find the immediate previous record (by createdAt)
	var previousRecord recordVersion
	if err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Select("id, createdAt").
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ? AND createdAt < ?",
			fid, fundId, fieldNo, formLineNo, currentRecord.CreatedAt).
		Order("createdAt DESC").
		Limit(1).
		First(&previousRecord).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[INFO] No previous version found - this is the first record created")
			return ErrNothingToDo
		}
		log.Printf("[ERROR] Failed to fetch previous record: %v", err)
		return err
	}

	now := time.Now()

	// 3. Deactivate the current version
	if err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Where("id = ?", currentRecord.ID).
		Updates(map[string]interface{}{
			"isCurrentValue": false,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error; err != nil {
		log.Printf("[ERROR] Failed to deactivate record id=%d: %v", currentRecord.ID, err)
		return err
	}

	// 4. Activate the previous version
	if err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Where("id = ?", previousRecord.ID).
		Updates(map[string]interface{}{
			"isCurrentValue": true,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error; err != nil {
		log.Printf("[ERROR] Failed to activate record id=%d: %v", previousRecord.ID, err)
		return err
	}

	log.Printf("[SUCCESS] Reverted from record id=%d to id=%d", currentRecord.ID, previousRecord.ID)
	return nil
}
