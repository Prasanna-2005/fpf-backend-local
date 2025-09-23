package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"

	"gorm.io/datatypes"
)

// BuildResponse produces the final JSON shape:
// { "funds": [ {fund_id, fund_name, fund_data}, … ] }
// It expects rows already ordered by fundSeriesId.
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

/* -------------------------------------------------
   helpers identical to your original edit.go
-------------------------------------------------*/

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

type FundInfo struct {
	FundID   string `json:"fund_id"`
	FundName string `json:"fund_name"`
	FundType string `json:"fund_type"`
}

type TagInfo struct {
	FundID   string `json:"fund_id"`
	XMLTags  string      `json:"xmltags"`
	Value    interface{} `json:"value"`
}

type SimpleResponse struct {
	FundInfo []FundInfo `json:"fund_info"`
	TagsInfo [][]TagInfo  `json:"tags_info"`
}

func TransformToSimpleResponse(filingID uint64) (*SimpleResponse, error) {
	rows, err := db.FetchFormDataByFiling(filingID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch form data: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no data found for filingID %d", filingID)
	}

	fundMap := make(map[string]FundInfo)
	tagMap := make(map[string][]TagInfo)

	for _, r := range rows {
		if _, exists := fundMap[r.FundID]; !exists {
			fundMap[r.FundID] = FundInfo{
				FundID:   r.FundID,
				FundName: r.FundName,
				FundType: r.FundType,
			}
		}

		tagMap[r.FundID] = append(tagMap[r.FundID], TagInfo{
			FundID:  r.FundID,
			XMLTags: string(r.XmlTags),
			Value:   r.Value,
		})
	}

	fundInfo := make([]FundInfo, 0, len(fundMap))
	tagsInfo := make([][]TagInfo, 0, len(tagMap))

	for fundID, f := range fundMap {
		fundInfo = append(fundInfo, f)
		tagsInfo = append(tagsInfo, tagMap[fundID])
	}

	return &SimpleResponse{
		FundInfo: fundInfo,
		TagsInfo: tagsInfo,
	}, nil
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
