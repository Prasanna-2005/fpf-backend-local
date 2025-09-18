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
		logger.Log("WARNING", "filing upload not completed")
		return nil, err
	}
	if len(rows) == 0 {
		logger.Log("WARNING", "Records not populated for filing")
		return nil, fmt.Errorf("no data for filing %d", filingID)
	}

	fundID, fundName, err := db.GetFundBrief(rows[0].FundSeriesID)
	if err != nil {

		return nil, err
	}

	grouped := make(map[string][]models.FormDatum)
	for _, r := range rows {
		grouped[r.FieldNo] = append(grouped[r.FieldNo], r)
	}

	resp := &models.FilingResponse{
		FundID:   fundID,
		FundName: fundName,
		FundData: make([]models.ResponseField, 0, len(grouped)),
	}

	for fieldNo, blockRows := range grouped {
		resp.FundData = append(resp.FundData, buildFieldBlock(fieldNo, blockRows))
	}
	return resp, nil
}

func buildFieldBlock(fieldNo string, rows []models.FormDatum) models.ResponseField {
	anyRow := rows[0]
	block := models.ResponseField{
		FieldNo:          fieldNo,
		HasFieldItemText: anyRow.HasFieldItemText,
		HasTableData:     anyRow.HasTableData,
		FieldText:        anyRow.LineText,
		FieldHeading:     anyRow.FieldHeading,
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    anyRow.IsMultipleAdd, // ← alias column exists
		FieldData:        make([]models.ResponseLineItem, 0, len(rows)),
	}

	// top-level value = concat **current** lines only
	for _, r := range rows {
		if r.IsCurrentValue {
			block.Value = appendAny(block.Value, ptrToInterface(r.Value))
		}
	}

	for _, r := range rows {
		if r.IsCurrentValue {
			block.FieldData = append(block.FieldData, buildLineItem(r))
		}
	}
	return block
}

func buildLineItem(r models.FormDatum) models.ResponseLineItem {
	return models.ResponseLineItem{
		LineItemNo:     r.FormLineNo,
		LineItemType:   r.LineItemType,
		LineText:       r.LineText,
		Value:          ptrToInterface(r.Value),
		Options:        jsonToSlice(r.Options), // datatypes.JSON → []string
		TableData:      nil,                    // delayed
		RequireDate:    r.RequireDate,
		DateValue:      jsonToInterface(r.DateValue), // datatypes.JSON → interface{}
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
