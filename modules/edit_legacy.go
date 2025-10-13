// modules/edit_legacy.go
package modules

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"sort"
	"strings"
)

type group struct {
	parent   models.FormFieldMSLegacy   // row whose formLineItemNo == fieldNo
	children []models.FormFieldMSLegacy // sub-lines 7_a, 7_b, …
}

/* ---------- public entry point ---------- */
func BuildResponselegacy(filingID uint64) (*models.FilingResponselegacy, error) {
	meta, err := db.FetchFormFieldMSLegacy()
	if err != nil {
		return nil, err
	}

	fsIDs, err := db.FetchFundSeriesForFiling(filingID)
	if err != nil {
		return nil, err
	}
	if len(fsIDs) == 0 {
		return &models.FilingResponselegacy{Funds: []models.Fundlegacy{}},
			fmt.Errorf("extraction is still in process")
	}

	valueMap, err := db.FetchCurrentValuesForFiling(filingID)
	if err != nil {
		return nil, err
	}

	fundMap, err := db.GetFundMap(fsIDs)
	if err != nil {
		return nil, err
	}

	/* ---------- 1. group meta rows by fieldNo ---------- */
	fieldMap := make(map[string]*group, 30)
	for _, m := range meta {
		if m.FormLineItemNo == m.FieldNo { // parent row
			if g := fieldMap[m.FieldNo]; g == nil {
				fieldMap[m.FieldNo] = &group{parent: m}
			} else {
				g.parent = m
			}
			continue
		}
		// child row
		if fieldMap[m.FieldNo] == nil {
			fieldMap[m.FieldNo] = &group{}
		}
		fieldMap[m.FieldNo].children = append(fieldMap[m.FieldNo].children, m)
	}

	/* ---------- 2. build response ---------- */
	resp := &models.FilingResponselegacy{Funds: make([]models.Fundlegacy, 0, len(fsIDs))}

	for _, fsID := range fsIDs {
		fundRec := fundMap[fsID]
		fund := models.Fundlegacy{
			FundID:   fundRec.FundID,
			FundName: fundRec.FundName,
		}

		for _, m := range meta {
			if len(fund.FundData) > 0 && fund.FundData[len(fund.FundData)-1].FieldNo == m.FieldNo {
				continue // already handled
			}
			g := fieldMap[m.FieldNo]
			switch m.DataType {
			case "text":
				fund.FundData = append(fund.FundData, buildFieldItemGroup(g, valueMap))
			case "bool":
				fund.FundData = append(fund.FundData, buildFieldItemGroup(g, valueMap))
			case "table":
				fund.FundData = append(fund.FundData, buildTableFieldItemGroup(m.FieldNo, fieldMap[m.FieldNo], valueMap))
			}
		}
		resp.Funds = append(resp.Funds, fund)
	}

	/* ---------- 3. sort funds by name ---------- */
	sort.Sort(byFundName(resp.Funds))

	/* ---------- 4. sort field_data inside every fund by field_no ---------- */
	for i := range resp.Funds {
		sort.Sort(byFieldNo(resp.Funds[i].FundData))
	}

	return resp, nil
}

/* ---------- universal table builder ---------- */
func buildTableFieldItemGroup(fieldNo string, g *group, vm map[string]string) models.FieldItem {
	// 1. parent row (the one whose formLineItemNo == fieldNo)
	par := g.parent
	if par.FormLineItemNo == "" { // fallback if parent not found
		for _, v := range g.children {
			if v.FormLineItemNo == fieldNo {
				par = v
				break
			}
		}
	}

	// 2. group children by row-key (second token after first underscore)
	//    never allow the parent key itself to slip in.
	rowMap := make(map[string]map[string]string) // rowKey -> columnKey -> value
	for _, v := range g.children {
		if v.FormLineItemNo == fieldNo { //always ignore the parent key
			continue
		}
		tokens := strings.SplitN(v.FormLineItemNo, "_", 3) // 17_a_xxx -> [17,a,xxx]
		// fmt.Printf("tokens:%v len:%v \n", tokens, len(tokens))
		if len(tokens) <= 2 {
			continue
		}
		rowKey, colKey := tokens[1], tokens[2]
		if rowMap[rowKey] == nil {
			rowMap[rowKey] = make(map[string]string)
		}

		rowMap[rowKey][colKey] = vm[v.FormLineItemNo]

	}
	// 3. stable row order (a,b,c...r for 17, assets/liabilities for 14, etc.)
	rowKeys := make([]string, 0, len(rowMap))
	for k := range rowMap {
		rowKeys = append(rowKeys, k)
	}
	sort.Strings(rowKeys)

	// 4. convert to final objects
	tableData := make([]map[string]interface{}, 0, len(rowMap))
	for _, rk := range rowKeys {
		var label string
		for _, v := range g.children {
			if strings.HasPrefix(v.FormLineItemNo, fieldNo+"_"+rk+"_") {
				label = v.LineText
				break
			}
		}
		obj := map[string]interface{}{
			"s.name": rk,
			"s.text": label,
		}
		for ck, val := range rowMap[rk] {
			// fmt.Printf("ck:%v val:%v\n", ck, val)
			obj[ck] = val
		}
		tableData = append(tableData, obj)
	}

	// 5. assemble FieldItem
	return models.FieldItem{
		FieldNo:          fieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     true,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(fieldNo),
		Value:            vm[fieldNo],
		// FieldData: []models.LineDetail{{
		// 	LineItemNo:     par.FormLineItemNo,
		// 	LineItemType:   par.DataType,
		// 	LineText:       "",
		// 	Value:          vm[fieldNo],
		// 	Options:        nil,
		// 	TableData:      tableData,
		// 	DisclaimerText: "",
		// }},
		FieldData:      []models.LineDetail{{}},
		TableData:      tableData,
		Confirmed:      false,
		Editing:        true,
		IsMultipleAdd:  par.IsMultipleAdd,
		DisclaimerText: par.DisclaimerText,
	}
}

/* ---------- helpers for text / bool ---------- */
func buildFieldItemGroup(g *group, vm map[string]string) models.FieldItem {
	par := g.parent
	fItem := models.FieldItem{
		FieldNo:          par.FieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     par.IsTableData,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(par.FieldNo),
		Value:            vm[par.FormLineItemNo],
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
		Options:          optionSlice(par.DataType),
	}

	fItem.FieldData = make([]models.LineDetail, 0, len(g.children))
	for _, l := range g.children {
		fItem.FieldData = append(fItem.FieldData, models.LineDetail{
			LineItemNo:     l.FormLineItemNo,
			LineItemType:   l.DataType,
			LineText:       l.LineText,
			Value:          vm[l.FormLineItemNo],
			Options:        optionSlice(l.DataType),
			TableData:      nil,
			RequireDate:    l.RequiresDate,
			DateValue:      "",
			DisclaimerText: l.DisclaimerText,
		})
	}
	return fItem
}

func optionSlice(dataType string) []string {
	if dataType == "bool" {
		return []string{"YES", "NO"}
	}
	return nil
}

func headingFor(fieldNo string) string {
	switch fieldNo {
	case "5", "6", "7":
		return "Item A. Reporting fund identifying information"
	case "8", "9", "10", "11", "12", "13", "14", "15", "16", "17":
		return "Item B. Assets, financing and investor concentration"
	default:
		return ""
	}
}
