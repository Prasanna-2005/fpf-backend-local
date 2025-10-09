package modules

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"sort"
)

type group struct {
	parent   models.FormFieldMSLegacy   // row whose formLineNo == fieldNo
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

		/* datatype-wise loop – emit only ONE FieldItem per fieldNo */
		for _, m := range meta {
			if len(fund.FundData) > 0 && fund.FundData[len(fund.FundData)-1].FieldNo == m.FieldNo {
				continue // already handled
			}
			g := fieldMap[m.FieldNo]
			switch m.DataType {
			case "text":
				fund.FundData = append(fund.FundData, buildFieldItemGroup(g, valueMap))
			case "options":
				fund.FundData = append(fund.FundData, buildFieldItemGroup(g, valueMap))
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

/* ---------- helper: build one FieldItem (parent + children) ---------- */
func buildFieldItemGroup(g *group, vm map[string]string) models.FieldItem {
	par := g.parent
	fItem := models.FieldItem{
		FieldNo:          par.FieldNo,
		HasFieldItemText: par.LineText != "", // false when parent text empty
		FieldItemType:    par.DataType,       // correct type
		HasTableData:     par.IsTableData,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(par.FieldNo),
		Value:            vm[par.FormLineItemNo],
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
		Options:          optionSlice(par.DataType), // nil unless parent is options
	}

	fItem.FieldData = make([]models.LineDetail, 0, len(g.children))
	for _, l := range g.children {
		fItem.FieldData = append(fItem.FieldData, models.LineDetail{
			LineItemNo:     l.FormLineItemNo,
			LineItemType:   l.DataType,
			LineText:       l.LineText,
			Value:          vm[l.FormLineItemNo],
			Options:        optionSlice(l.DataType), // only 7_a gets YES/NO
			TableData:      nil,
			RequireDate:    l.RequiresDate,
			DateValue:      "",
			DisclaimerText: l.DisclaimerText,
		})
	}
	return fItem
}

/* ---------- tiny utilities ---------- */
func optionSlice(dataType string) []string {
	if dataType == "options" {
		return []string{"YES", "NO"}
	}
	return nil
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
