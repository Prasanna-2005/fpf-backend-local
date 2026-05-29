package db

import (
	"errors"
	"fmt"
	"form_pf_backend/models"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
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
	Fundtype string
}

func loadFundCache() {
	all, _ := GetAllFunds()
	m := make(map[string]fundBrief, len(all))
	for _, f := range all {
		m[f.FundID] = fundBrief{FundID: f.FundID, FundName: f.FundName, Fundtype: f.FundType}
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

func FetchFormDataByFiling(filingID string) ([]models.FormDatum, error) {
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
	for _, pair := range pairs {
		if slices.Contains(AUM, pair.ln) {
			UpdateAUM(fid, fundId, uid, pair.ln, pair.vl)
		}
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
			ID:             uuid.New().String(),
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

	// 1. filing exists
	var cnt int64
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ?", fid).Limit(1).Count(&cnt).Error; err != nil || cnt == 0 {
		return ErrFilingNotFound
	}

	// 2. collect every (fieldNo, formLineNo) we must revert
	type target struct{ fieldNo, lineNo, value string }
	targets := make([]target, 0, 8)

	if data.HasTableData && data.TableData != nil {
		rows, err := castTableData(data.TableData)
		if err != nil {
			return err
		}
		for ln, val := range explodeTableCells(data.FieldNo, rows) {
			targets = append(targets, target{fieldNo: data.FieldNo, lineNo: ln, value: val})
		}
	} else {
		if data.Value != "" {
			targets = append(targets, target{fieldNo: data.FieldNo, lineNo: data.FieldNo, value: data.Value})
		}
		for _, fd := range data.FieldData {
			targets = append(targets, target{fieldNo: data.FieldNo, lineNo: fd.LineItemNo, value: fd.Value})
		}
	}
	if len(targets) == 0 {
		return ErrNothingToDo
	}

	// 3. bulk fetch CURRENT rows (isCurrentValue = 1)
	fieldNos := make([]string, len(targets))
	lineNos := make([]string, len(targets))
	for i, t := range targets {
		fieldNos[i] = t.fieldNo
		lineNos[i] = t.lineNo
		if slices.Contains(AUM, t.fieldNo) {
			UpdateAUM(fid, fundId, uid, t.lineNo, "0")
		}
	}

	type curRow struct {
		ID         string    `gorm:"column:id"`
		FieldNo    string    `gorm:"column:fieldNo"`
		FormLineNo string    `gorm:"column:formLineNo"`
		CreatedAt  time.Time `gorm:"column:createdAt"`
	}
	var currents []curRow

	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Select("id", "fieldNo", "formLineNo", "createdAt").
		Where("filingid = ? AND fundSeriesId = ? AND isCurrentValue = ? AND fieldNo IN ? AND formLineNo IN ?",
			fid, fundId, true, fieldNos, lineNos).
		Find(&currents).Error; err != nil {
		return err
	}
	if len(currents) == 0 {
		return ErrNothingToDo
	}

	// 4. bulk fetch IMMEDIATE PREDECESSOR for every current row
	//    (biggest created_at < current.createdAt per fieldNo+formLineNo)
	type oldRow struct {
		ID         string `gorm:"column:id"`
		FieldNo    string `gorm:"column:fieldNo"`
		FormLineNo string `gorm:"column:formLineNo"`
	}
	var olds []oldRow

	// build OR-clause of (fieldNo, formLineNo, createdAt) tuples
	cond := sqlDb.Session(&gorm.Session{NewDB: true}) // fresh builder
	for _, c := range currents {
		cond = cond.Or("(fieldNo = ? AND formLineNo = ? AND createdAt < ?)",
			c.FieldNo, c.FormLineNo, c.CreatedAt)
	}

	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Select("id", "fieldNo", "formLineNo", "createdAt").
		Where("filingid = ? AND fundSeriesId = ?", fid, fundId).
		Where(cond).
		Order("fieldNo, formLineNo, createdAt DESC"). // important
		Find(&olds).Error; err != nil {
		return err
	}

	// 5. match each current row to its immediate predecessor
	reverts := make([]struct{ curID, oldID string }, 0, len(currents))
	for _, c := range currents {
		for _, o := range olds {
			if o.FieldNo == c.FieldNo && o.FormLineNo == c.FormLineNo && o.ID != c.ID {
				reverts = append(reverts, struct{ curID, oldID string }{curID: c.ID, oldID: o.ID})
				break // first hit is the latest one because of ORDER BY
			}
		}
	}
	if len(reverts) == 0 {
		return ErrNothingToDo
	}

	// 6. bulk switch OFF current versions
	curIDs := make([]string, len(reverts))
	for i, p := range reverts {
		curIDs[i] = p.curID
	}
	now := time.Now()
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("id IN ?", curIDs).
		Updates(map[string]interface{}{
			"isCurrentValue": false,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error; err != nil {
		return err
	}

	// 7. bulk switch ON previous versions
	oldIDs := make([]string, len(reverts))
	for i, p := range reverts {
		oldIDs[i] = p.oldID
	}
	return sqlDb.Model(&models.FormDataLegacy{}).
		Where("id IN ?", oldIDs).
		Updates(map[string]interface{}{
			"isCurrentValue": true,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error
}
