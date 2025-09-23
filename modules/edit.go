package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"sort"
	"strconv"
	"strings"
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

	var curFund *models.FundEntry
	fieldMap := make(map[string][]models.FormDatum)

	flush := func() {
		if curFund == nil {
			return
		}
		// build field blocks for the fund we just finished
		for fieldNo, blockRows := range fieldMap {
			curFund.FundData = append(curFund.FundData, buildFieldBlock(fieldNo, blockRows))
		}

		sort.Slice(curFund.FundData, func(i, j int) bool {
			fi, _ := strconv.Atoi(curFund.FundData[i].FieldNo)
			fj, _ := strconv.Atoi(curFund.FundData[j].FieldNo)
			return fi < fj
		})

		// 2.  sort line items by *numeric* line_item_no
		for i := range curFund.FundData {
			sort.Slice(curFund.FundData[i].FieldData, func(k, l int) bool {
				// strip the underscore suffix if you have “5_a”, “5_b”, etc.
				lk := strings.Split(curFund.FundData[i].FieldData[k].LineItemNo, "_")[0]
				ll := strings.Split(curFund.FundData[i].FieldData[l].LineItemNo, "_")[0]
				nk, _ := strconv.Atoi(lk)
				nl, _ := strconv.Atoi(ll)
				return nk < nl
			})
		}

		resp.Funds = append(resp.Funds, *curFund)
		fieldMap = make(map[string][]models.FormDatum)
	}

	for _, r := range rows {
		//forbidden table fields
		if r.FieldNo == "20" || r.FieldNo == "23" {
			continue
		}

		// new fund detected  (rows ordered by fundSeriesId)
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

/* ----------  helper functions (unchanged)  ---------- */

func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func fallbackDate() string {
	return fmt.Sprintf("09/30/%d", time.Now().Year())
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

	fieldDisclaimer := safeString(&anyRow.DisclaimerText)
	fieldHeading := safeString(&anyRow.FieldHeading)

	block := models.ResponseField{
		FieldNo:          fieldNo,
		HasFieldItemText: hasFieldItemText,
		HasTableData:     anyRow.HasTableData,
		FieldText:        fieldText,
		FieldHeading:     fieldHeading,
		Value:            nil,
		FieldData:        []models.ResponseLineItem{},
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    anyRow.IsMultipleAdd,
		DisclaimerText:   fieldDisclaimer,
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
		DisclaimerText: parentDisclaimer,
	}
}
