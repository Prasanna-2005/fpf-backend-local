// modules/edit.go  (full file – text-only, skeleton-first, uses your Fund model)
package modules

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
)

func BuildResponselegacy(filingID uint64) (*models.FilingResponselegacy, error) {
	// 1. load meta
	meta, err := db.FetchFormFieldMSLegacy()
	if err != nil {
		return nil, err
	}

	// 2. fund-series that belong to this filing
	fsIDs, err := db.FetchFundSeriesForFiling(filingID)
	if err != nil {
		return nil, err
	}
	if len(fsIDs) == 0 {
		return &models.FilingResponselegacy{Funds: []models.Fundlegacy{}}, fmt.Errorf("extraction is still in process")
	}

	// 2½. one query to grab all fund names/ids
	fundMap, err := db.GetFundMap(fsIDs)
	if err != nil {
		return nil, err
	}

	resp := &models.FilingResponselegacy{Funds: make([]models.Fundlegacy, 0, len(fsIDs))}

	// 3. build one Fund block per series
	for _, fsID := range fsIDs {
		fundRec, ok := fundMap[fsID]
		if !ok { // should never happen
			continue
		}
		fund := models.Fundlegacy{
			FundID:   fundRec.FundID,
			FundName: fundRec.FundName,
		}

		// 4. convert every meta row into FieldItem (text only)
		for _, m := range meta {
			if m.DataType == "text" {
				fund.FundData = append(fund.FundData, buildTextFieldItem(m, fsID))
			}
		}
		resp.Funds = append(resp.Funds, fund)
	}
	return resp, nil
}

func buildTextFieldItem(meta models.FormFieldMSLegacy, fsID uint64) models.FieldItem {
	val, _ := db.FetchCurrentValue(fsID, meta.FormLineItemNo)

	return models.FieldItem{
		FieldNo:          meta.FieldNo,
		HasFieldItemText: meta.LineText != "",
		FieldItemType:    "text",
		HasTableData:     false,
		FieldText:        meta.LineText,
		FieldHeading:     headingFor(meta.FieldNo), // tiny mapper you already have
		Value:            val,
		FieldData:        buildTextLineDetails(meta, fsID),
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    meta.IsMultipleAdd,
		DisclaimerText:   meta.DisclaimerText,
	}
}

func buildTextLineDetails(meta models.FormFieldMSLegacy, fsID uint64) []models.LineDetail {
	val, _ := db.FetchCurrentValue(fsID, meta.FormLineItemNo)
	return []models.LineDetail{{
		LineItemNo:     meta.FormLineItemNo,
		LineItemType:   "text",
		LineText:       meta.LineText,
		Value:          val,
		Options:        nil,
		TableData:      nil,
		RequireDate:    meta.RequiresDate,
		DateValue:      "",
		DisclaimerText: meta.DisclaimerText,
	}}
}

func headingFor(fieldNo string) string {
	switch fieldNo {
	case "5", "6", "7":
		return "Item A. Reporting fund identifying information"
	case "8", "9", "10", "11", "12", "13", "14", "15":
		return "Item B. Assets, financing and investor concentration"
	default:
		return ""
	}
}
