package db

import (
	"errors"
	"fmt"
	"form_pf_backend/models"
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
		Table("form_fields_ms fld").
		Select(`d.createdAt, d.createdBy, d.updatedAt, d.updatedBy, d.value,
		        d.fundSeriesId, d.date,
		        f.fundName, f.fundType,
		        fld.fieldNo, fld.formLineNo,fld.fieldHeading, fld.lineText, fld.dataType,
		        fld.isMultipleAdd, fld.isMSQ, fld.isMCQ, fld.requiresDate,
		        fld.isTableData, fld.disclaimerText, fld.xmlTags`).
		Joins("LEFT JOIN form_data d ON fld.fieldNo = d.fieldNo AND "+
			"fld.formLineNo = d.formLineNo AND "+
			"d.filingId = ? AND d.isCurrentValue = 1", filingID).
		Joins("INNER JOIN feg_funds f ON f.fegFundId = d.fundSeriesId").
		Order(`d.fundSeriesId IS NULL, d.fundSeriesId, fld.fieldNo, fld.formLineNo`). // NULLs last, then by fund, then field/line
		Limit(400).
		Find(&rows).Error
	return rows, err
}

// func UpdateLineItem(data *models.ResponseField, fid, uid, fundId string) error {
// 	sqlDb := GetDB()
// 	ok := dbAlive(sqlDb)
// 	if !ok {
// 		return ServerError
// 	}

// 	var count int64
// 	err := sqlDb.Model(&models.FormDataLegacy{}).Where("filingid = ?", fid).Count(&count).Error
// 	if err != nil {
// 		return err
// 	}
// 	if count == 0 {
// 		return ErrFilingNotFound
// 	}

// 	// First check outside value case - formLineNo = fieldNo
// 	// Skip if outside value is empty or null
// 	hasOutsideValue := false

// 	// Replace these field names with actual ones from ResponseField model
// 	if data.Value != nil { // Assuming there's a Value field for outside values
// 		if v, ok := data.Value.(string); ok && v != "" {
// 			hasOutsideValue = true
// 		}
// 	}

// 	if hasOutsideValue {
// 		err := updateSingleRecord(sqlDb, data, fid, uid, fundId, true) // true for outside value
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	// Then check all FieldData items
// 	for i := range data.FieldData {
// 		err := updateSingleRecord(sqlDb, data, fid, uid, fundId, false, i) // false for inside, with index
// 		if err != nil || errors.Is(err, ErrNothingToDo) {
// 			return err
// 		}
// 	}

// 	return nil
// }

// func updateSingleRecord(sqlDb *gorm.DB, data *models.ResponseField, fid, uid, fundId string, isOutside bool, fieldIndex ...int) error {
// 	fmt.Println(uid)
// 	var targetLineNo string
// 	var value, dateValue interface{}

// 	if isOutside {
// 		// Outside value case - formLineNo = fieldNo
// 		targetLineNo = data.FieldNo
// 		value = data.Value
// 		// dateValue = data.OutsideDateValue // uncomment if field exists
// 	} else {
// 		// Inside FieldData case
// 		idx := 0
// 		if len(fieldIndex) > 0 {
// 			idx = fieldIndex[0]
// 		}
// 		targetLineNo = data.FieldData[idx].LineItemNo
// 		value = data.FieldData[idx].Value
// 		dateValue = data.FieldData[idx].DateValue
// 	}
// 	// Extract new values
// 	var newValue, newDate string

// 	if value != nil {
// 		if v, ok := value.(string); ok && v != "" {
// 			newValue = v
// 		}
// 	}
// 	if dateValue != nil {
// 		if dv, ok := dateValue.(string); ok && dv != "" {
// 			newDate = dv
// 		}
// 	}
// 	fmt.Println(newDate)
// 	// Find the existing record
// 	var old models.FormDataLegacy
// 	err := sqlDb.Where(map[string]interface{}{
// 		"filingid":       fid,
// 		"fundSeriesId":   fundId,
// 		"fieldNo":        data.FieldNo,
// 		"formLineNo":     targetLineNo,
// 		"isCurrentValue": true,
// 	}).First(&old).Error

// 	if err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return gorm.ErrRecordNotFound
// 		}
// 		return err
// 	}

// 	// Check if there's actually something to update
// 	// valueSame := (old.Value == nil && newValue == nil) ||
// 	// 	(old.Value != nil && newValue != nil && *old.Value == *newValue)
// 	// dateSame := (old.Date == nil && newDate == nil) ||
// 	// 	(old.Date != nil && newDate != nil && *old.Date == *newDate)

// 	// if valueSame && dateSame {
// 	// 	return ErrNothingToDo
// 	// }

// 	// Update existing record to mark as not current
// 	now := time.Now()
// 	if err := sqlDb.Model(&old).
// 		Select("isCurrentValue", "updatedAt", "updatedBy").
// 		Updates(models.FormDataLegacy{
// 			IsCurrentValue: false,
// 			UpdatedAt:      now,
// 			UpdatedBy:      uid,
// 		}).Error; err != nil {
// 		return err
// 	}

// 	// Create new record with updated values
// 	newRow := models.FormDataLegacy{
// 		FilingID:       fid,
// 		FundSeriesid:   fundId,
// 		FieldNo:        data.FieldNo,
// 		FormLineNo:     targetLineNo,
// 		Value:          newValue,
// 		IsCurrentValue: true,
// 		CreatedBy:      uid,
// 		CreatedAt:      now,
// 		UpdatedBy:      uid,
// 	}

// 	return sqlDb.Create(&newRow).Error
// }

// func RevertLineItem(data *models.ResponseField, fid, uid, fundId string) error {
// 	sqlDb := GetDB()
// 	if !dbAlive(sqlDb) {
// 		return ServerError
// 	}

// 	var count int64
// 	if err := sqlDb.Model(&models.FormDataLegacy{}).Where("filingId = ?", fid).Count(&count).Error; err != nil {
// 		return err
// 	}
// 	if count == 0 {
// 		return ErrFilingNotFound
// 	}

// 	//outside value handling
// 	if data.Value != nil {
// 		if err := RevertSingleRecord(sqlDb, data, fid, uid, fundId, true); err != nil && !errors.Is(err, ErrNothingToDo) {
// 			return err
// 		}
// 	}

// 	for i := range data.FieldData {
// 		if err := RevertSingleRecord(sqlDb, data, fid, uid, fundId, false, i); err != nil && !errors.Is(err, ErrNothingToDo) {
// 			return err
// 		}
// 	}

// 	return nil
// }

// func RevertSingleRecord(sqlDb *gorm.DB, data *models.ResponseField, fid, uid, fundId string, isOutside bool, fieldIndex ...int) error {
// 	var targetLineNo string

// 	if isOutside {
// 		targetLineNo = data.FieldNo
// 	} else {
// 		idx := 0
// 		if len(fieldIndex) > 0 {
// 			idx = fieldIndex[0]
// 		}
// 		targetLineNo = data.FieldData[idx].LineItemNo
// 	}

// 	// --- Begin transaction ---
// 	tx := sqlDb.Begin()
// 	defer func() {
// 		if r := recover(); r != nil {
// 			tx.Rollback()
// 		}
// 	}()

// 	var rows []models.FormDataLegacy
// 	if err := tx.Where("filingId = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ?",
// 		fid, fundId, data.FieldNo, targetLineNo).
// 		Order("createdAt DESC").
// 		Find(&rows).Error; err != nil {
// 		tx.Rollback()
// 		return err
// 	}

// 	if len(rows) == 0 {
// 		tx.Rollback()
// 		return ErrNothingToDo
// 	}

// 	var currentIdx int = -1
// 	for i, r := range rows {
// 		if r.IsCurrentValue {
// 			currentIdx = i
// 			break
// 		}
// 	}
// 	if currentIdx == -1 {
// 		currentIdx = 0
// 	}

// 	if currentIdx+1 >= len(rows) {
// 		tx.Rollback()
// 		return ErrNothingToDo
// 	}

// 	prev := &rows[currentIdx+1]
// 	now := time.Now()

// 	//  mark all rows for this field as not current
// 	if err := tx.Model(&models.FormDataLegacy{}).
// 		Where("filingId = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ?",
// 			fid, fundId, data.FieldNo, targetLineNo).
// 		Update("isCurrentValue", false).Error; err != nil {
// 		tx.Rollback()
// 		return err
// 	}

// 	// mark the previous record as current
// 	if err := tx.Model(&models.FormDataLegacy{}).
// 		Where("id = ?", prev.ID).
// 		Updates(map[string]interface{}{
// 			"isCurrentValue": true,
// 			"updatedAt":      now,
// 			"updatedBy":      uid,
// 		}).Error; err != nil {
// 		tx.Rollback()
// 		return err
// 	}

// 	return tx.Commit().Error
// }

// ---------- 1.  UpdateLineItem  (transaction_db.go) ----------
func UpdateLineItem(data *models.FieldItem, fid, uid, fundId string) error {
	sqlDb := GetDB()
	if !dbAlive(sqlDb) {
		return ServerError
	}

	/* ----- 0.  filing exists? ----- */
	var cnt int64
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ?", fid).Count(&cnt).Error; err != nil || cnt == 0 {
		return ErrFilingNotFound
	}

	/* ----- 1.  TABLE  (many rows) ----- */
	if data.HasTableData && data.TableData != nil {
		rows, err := castTableData(data.TableData)
		if err != nil {
			return err
		}
		cells := explodeTableCells(data.FieldNo, rows)
		for formLineNo, cellValue := range cells {
			tableCell := map[string]string{"ln": formLineNo, "vl": cellValue}
			if err := updateSingleRecord(sqlDb, data, fid, uid, fundId,
				true, tableCell); err != nil && !errors.Is(err, ErrNothingToDo) {
				return err
			}
		}
		return nil
	}

	/* ----- 2.  BOOL / TEXT / OPTIONS  (single rows) ----- */
	if data.Value != "" {
		_ = updateSingleRecord(sqlDb, data, fid, uid, fundId, true, nil)
	}
	for i := range data.FieldData {
		_ = updateSingleRecord(sqlDb, data, fid, uid, fundId, false, nil, i)
	}
	return nil
}

// ---------- 2.  updateSingleRecord  (transaction_db.go) ----------
func updateSingleRecord(sqlDb *gorm.DB, data *models.FieldItem,
	fid, uid, fundId string, isOutside bool, tableCell map[string]string, fieldIndex ...int) error {

	var targetLineNo, newValue string

	switch {
	case isOutside && data.HasTableData:
		/* TABLE – strings come from the captured map */
		if tableCell != nil {
			targetLineNo = tableCell["ln"]
			newValue = tableCell["vl"]
		}

	case isOutside && !data.HasTableData:
		/* classic field-level value */
		targetLineNo = data.FieldNo
		newValue = data.Value

	default:
		/* inside FieldData line */
		idx := 0
		if len(fieldIndex) > 0 {
			idx = fieldIndex[0]
		}
		targetLineNo = data.FieldData[idx].LineItemNo
		newValue = data.FieldData[idx].Value
	}

	now := time.Now()
	// mark old row(s) not-current
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ? AND isCurrentValue = ?",
			fid, fundId, data.FieldNo, targetLineNo, true).
		Update("isCurrentValue", false).Error; err != nil {
		return err
	}

	newRow := models.FormDataLegacy{
		FilingID:       fid,
		FundSeriesid:   fundId,
		FieldNo:        data.FieldNo,
		FormLineNo:     targetLineNo,
		Value:          newValue,
		IsCurrentValue: true,
		CreatedBy:      uid,
		CreatedAt:      now,
		UpdatedBy:      uid,
	}
	return sqlDb.Create(&newRow).Error
}

// ---------- 3.  explodeTableCells  (transaction_db.go) ----------
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

/* ---------- safe table extractor ---------- */
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

	/* ----- 0.  filing exists? ----- */
	var cnt int64
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ?", fid).Count(&cnt).Error; err != nil || cnt == 0 {
		return ErrFilingNotFound
	}

	/* ----- 1.  TABLE  (many rows) ----- */
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

	/* ----- 2.  BOOL / TEXT / OPTIONS  (single rows) ----- */
	/* outside value */
	if data.Value != "" {
		if err := revertSingleRecord(sqlDb, fid, uid, fundId,
			data.FieldNo, data.FieldNo); err != nil && !errors.Is(err, ErrNothingToDo) {
			return err
		}
	}
	/* inside field_data lines */
	for _, line := range data.FieldData {
		if err := revertSingleRecord(sqlDb, fid, uid, fundId,
			data.FieldNo, line.LineItemNo); err != nil && !errors.Is(err, ErrNothingToDo) {
			return err
		}
	}
	return nil
}

func revertSingleRecord(sqlDb *gorm.DB, fid, uid, fundId, fieldNo, formLineNo string) error {
	tx := sqlDb.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var rows []models.FormDataLegacy
	if err := tx.Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ?",
		fid, fundId, fieldNo, formLineNo).
		Order("createdAt DESC").
		Find(&rows).Error; err != nil {
		tx.Rollback()
		return err
	}
	if len(rows) == 0 {
		tx.Rollback()
		return ErrNothingToDo
	}

	// find current row
	curr := -1
	for i, r := range rows {
		if r.IsCurrentValue {
			curr = i
			break
		}
	}
	if curr == -1 { // nothing marked current → treat newest as current
		curr = 0
	}
	if curr+1 >= len(rows) { // no older row to restore
		tx.Rollback()
		return ErrNothingToDo
	}

	prev := &rows[curr+1]
	now := time.Now()

	// mark all versions of this cell/line not-current
	if err := tx.Model(&models.FormDataLegacy{}).
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ?",
			fid, fundId, fieldNo, formLineNo).
		Update("isCurrentValue", false).Error; err != nil {
		tx.Rollback()
		return err
	}

	// restore the previous version
	if err := tx.Model(&models.FormDataLegacy{}).
		Where("id = ?", prev.ID).
		Updates(map[string]interface{}{
			"isCurrentValue": true,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}
