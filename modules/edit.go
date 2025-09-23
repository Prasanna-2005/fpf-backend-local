package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"time"

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
		fieldMap = make(map[string][]models.FormDatum)
	}

	for _, r := range rows {
		if r.FieldNo == "20" || r.FieldNo == "23" {
			continue
		}
		if curFund == nil || curFund.FundID != r.FundID {
			flush()
			curFund = &models.FundEntry{
				FundID:   r.FundID,
				FundName: r.FundName,
				FundData: make([]models.ResponseField, 0, 64),
			}
		}

		fieldMap[r.FieldNo] = append(fieldMap[r.FieldNo], r)
	}
	flush()

	return resp, nil
}

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func fallbackDate() string {
	return fmt.Sprintf("09/30/%d", time.Now().Year())
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

func appendAny(dst interface{}, src interface{}) interface{} {
	if dst == nil {
		return src
	}
	return dst
}

func buildFieldBlock(fieldNo string, rows []models.FormDatum) models.ResponseField {
	anyRow := rows[0]
	isSummaryLine := anyRow.FormLineNo == anyRow.FieldNo
	fieldText := ""
	hasFieldItemText := false
	if isSummaryLine {
		if txt := safeString(&anyRow.LineText); txt != "" {
			fieldText = txt
			hasFieldItemText = true
		}
	}

	fieldDisclaimer := safeString(&anyRow.DisclaimerText) // nullable column

	block := models.ResponseField{
		FieldNo:          fieldNo,
		HasFieldItemText: hasFieldItemText,
		HasTableData:     anyRow.HasTableData,
		FieldText:        fieldText,
		FieldHeading:     anyRow.FieldHeading,
		Value:            nil,
		FieldData:        []models.ResponseLineItem{}, // always start empty
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    anyRow.IsMultipleAdd,
		DisclaimerText:   fieldDisclaimer, // <-- outside
	}

	for _, r := range rows {
		block.Value = appendAny(block.Value, safeString(r.Value))
	}

	if !isSummaryLine {
		for _, r := range rows {
			li := buildLineItem(r, fieldDisclaimer)
			block.FieldData = append(block.FieldData, li)
		}
	}

	return block
}

// buildLineItem  ---------------------------------------------------
func buildLineItem(r models.FormDatum, parentDisclaimer string) models.ResponseLineItem {
	requireDate := r.RequireDate
	dateVal := ""
	if requireDate || r.Date != nil {
		if r.Date != nil && *r.Date != "" {
			dateVal = *r.Date
		} else {
			dateVal = fallbackDate()
		}
		requireDate = true
	}

	return models.ResponseLineItem{
		LineItemNo:     r.FormLineNo,
		LineItemType:   r.LineItemType,
		LineText:       r.LineText,
		Value:          safeString(r.Value),
		Options:        jsonToSlice(r.Options),
		TableData:      nil,
		RequireDate:    requireDate,
		DateValue:      dateVal,
		DisclaimerText: parentDisclaimer, // same text as field level
	}
}
