package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"

	"gorm.io/datatypes"
)

func BuildResponse(filingID uint64) (*models.FilingResponse, error) {
	rows, err := db.FetchFormDataByFiling(filingID)
	if err != nil {
		logger.Log("WARNING", "Records not populated for filing")
		return nil, err
	}
	if len(rows) == 0 {
		logger.Log("WARNING", "Records not populated for filing")
		return nil, fmt.Errorf("no data for filing %d", filingID)
	}

	resp := &models.FilingResponse{
		Funds: make([]models.FundEntry, 0, 8),
	}

	var curFund *models.FundEntry // fund we are filling right now
	fieldMap := make(map[string][]models.FormDatum)

	flush := func() {
		if curFund == nil {
			return
		}
		// build field blocks for the fund we just finished
		for fieldNo, blockRows := range fieldMap {
			curFund.FundData = append(curFund.FundData, buildFieldBlock(fieldNo, blockRows))
		}
		// append completed fund to response
		resp.Funds = append(resp.Funds, *curFund)
		// reset for next fund
		fieldMap = make(map[string][]models.FormDatum)
	}

	for _, r := range rows {
		// new fund detected  (rows ordered by fundSeriesId)
		if curFund == nil || curFund.FundID != r.FundID {
			flush()
			curFund = &models.FundEntry{
				FundID:   r.FundID,
				FundName: r.FundName,
				FundData: make([]models.ResponseField, 0, 64),
			}
		}
		// accumulate rows into field groups
		fieldMap[r.FieldNo] = append(fieldMap[r.FieldNo], r)
	}
	flush() // don’t forget the last one

	return resp, nil
}

func buildFieldBlock(fieldNo string, rows []models.FormDatum) models.ResponseField {
	anyRow := rows[0]
	if len(anyRow.LineText) != 0 {
		anyRow.HasFieldItemText = true
	}
	block := models.ResponseField{
		FieldNo:          fieldNo,
		HasFieldItemText: anyRow.HasFieldItemText,
		HasTableData:     anyRow.HasTableData,
		FieldText:        anyRow.LineText,
		FieldHeading:     anyRow.FieldHeading,
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    anyRow.IsMultipleAdd,
		FieldData:        make([]models.ResponseLineItem, 0, len(rows)),
	}

	// top-level value = concat **current** lines only
	for _, r := range rows {
		block.Value = appendAny(block.Value, ptrToInterface(r.Value))
	}

	for _, r := range rows {
		block.FieldData = append(block.FieldData, buildLineItem(r))
	}
	return block
}

func buildLineItem(r models.FormDatum) models.ResponseLineItem {
	return models.ResponseLineItem{
		LineItemNo:     r.FormLineNo,
		LineItemType:   r.LineItemType,
		LineText:       r.LineText,
		Value:          ptrToInterface(r.Value),
		Options:        jsonToSlice(r.Options),
		TableData:      nil, // filled later if needed
		RequireDate:    r.RequireDate,
		DateValue:      jsonToInterface(r.DateValue),
		DisclaimerText: r.DisclaimerText,
	}
}

func jsonToSlice(j datatypes.JSON) []string {
	if len(j) == 0 {
		return nil
	}
	var s []string
	_ = json.Unmarshal(j, &s)
	return s
}

func jsonToInterface(j datatypes.JSON) interface{} {
	if len(j) == 0 {
		return ""
	}
	var v interface{}
	_ = json.Unmarshal(j, &v)
	return v
}

func ptrToInterface(s *string) interface{} {
	if s == nil {
		return ""
	}
	return *s
}

func appendAny(dst interface{}, src interface{}) interface{} {
	if dst == nil {
		return src
	}
	return dst
}
