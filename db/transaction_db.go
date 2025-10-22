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

// UpdateLineItem  (transaction_db.go)
func UpdateLineItem(data *models.FieldItem, fid, uid, fundId string) error {
	sqlDb := GetDB()
	if !dbAlive(sqlDb) {
		return ServerError
	}

	// filing exists
	var cnt int64
	if err := sqlDb.Model(&models.FormDataLegacy{}).
		Where("filingid = ?", fid).Count(&cnt).Error; err != nil || cnt == 0 {
		return ErrFilingNotFound
	}

	//.  TABLE  (many rows)
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

	//  BOOL / TEXT / OPTIONS  (single rows)
	if data.Value != "" {
		_ = updateSingleRecord(sqlDb, data, fid, uid, fundId, true, nil)
	}
	for i := range data.FieldData {
		_ = updateSingleRecord(sqlDb, data, fid, uid, fundId, false, nil, i)
	}
	return nil
}

// updateSingleRecord  (transaction_db.go)
func updateSingleRecord(sqlDb *gorm.DB, data *models.FieldItem,
	fid, uid, fundId string, isOutside bool, tableCell map[string]string, fieldIndex ...int) error {

	var targetLineNo, newValue string

	switch {
	case isOutside && data.HasTableData:
		// TABLE – strings come from the captured map
		if tableCell != nil {
			targetLineNo = tableCell["ln"]
			newValue = tableCell["vl"]
		}

	case isOutside && !data.HasTableData:
		// classic field-level value
		targetLineNo = data.FieldNo
		newValue = data.Value

	default:
		// inside FieldData line
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

	// 1.  load the two latest versions (we only need their IDs and flags)
	type mini struct {
		ID             uint `gorm:"column:id"`
		IsCurrentValue bool `gorm:"column:isCurrentValue"`
	}
	var rows []mini
	if err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Select("id, isCurrentValue").
		Where("filingid = ? AND fundSeriesId = ? AND fieldNo = ? AND formLineNo = ?",
						fid, fundId, fieldNo, formLineNo).
		Order("createdAt DESC"). // newest first
		Limit(2).
		Scan(&rows).Error; err != nil {
		return err
	}

	switch len(rows) {
	case 0:
		return ErrNothingToDo // no data at all
	case 1:
		return ErrNothingToDo // only one version – nothing to revert to
	}
	log.Printf("[rows:%d] %#v\n", len(rows), rows)
	fmt.Println(rows)
	// rows[0] = newest (the one we will turn OFF)
	// rows[1] = previous (the one we will turn ON)
	if !rows[0].IsCurrentValue {
		fmt.Println("hi0")
		return ErrNothingToDo // newest is already OFF → nothing to do
	}

	fmt.Printf("hi")
	now := time.Now()

	// 2.  turn OFF the current row (identified by its PK)
	if err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Where("id = ?", rows[0].ID).
		Updates(map[string]interface{}{
			"isCurrentValue": false,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error; err != nil {
		fmt.Println(err.Error())
		return err
	}
	fmt.Println("hi1")

	// 3.  turn ON the previous row (identified by its PK)
	err := sqlDb.
		Model(&models.FormDataLegacy{}).
		Where("id = ?", rows[1].ID).
		Updates(map[string]interface{}{
			"isCurrentValue": true,
			"updatedAt":      now,
			"updatedBy":      uid,
		}).Error
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println("hi2")
	return err
}
