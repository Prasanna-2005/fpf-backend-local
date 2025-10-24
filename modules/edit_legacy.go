// modules/edit_legacy.go
package modules

import (
	"encoding/json"
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"sort"
	"strings"
)

type group struct {
	parent   models.FormFieldMSLegacy
	children []models.FormFieldMSLegacy
}

// OrderedMap is a simple ordered map implementation for JSON marshaling
type OrderedMap struct {
	keys   []string
	values map[string]interface{}
}

func NewOrderedMap() *OrderedMap {
	return &OrderedMap{
		keys:   make([]string, 0),
		values: make(map[string]interface{}),
	}
}

func (om *OrderedMap) Set(key string, value interface{}) {
	if _, exists := om.values[key]; !exists {
		om.keys = append(om.keys, key)
	}
	om.values[key] = value
}

func (om *OrderedMap) MarshalJSON() ([]byte, error) {
	result := "{"
	for i, k := range om.keys {
		if i > 0 {
			result += ","
		}
		keyJSON, _ := json.Marshal(k)
		valJSON, _ := json.Marshal(om.values[k])
		result += string(keyJSON) + ":" + string(valJSON)
	}
	result += "}"
	return []byte(result), nil
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

	// / 1. group meta rows by fieldNo
	fieldMap := make(map[string]*group, 30)
	for _, m := range meta {
		if m.FormLineItemNo == m.FieldNo {
			if g := fieldMap[m.FieldNo]; g == nil {
				fieldMap[m.FieldNo] = &group{parent: m}
			} else {
				g.parent = m
			}
			continue
		}
		if fieldMap[m.FieldNo] == nil {
			fieldMap[m.FieldNo] = &group{}
		}
		fieldMap[m.FieldNo].children = append(fieldMap[m.FieldNo].children, m)
	}

	// 2. build response
	resp := &models.FilingResponselegacy{Funds: make([]models.Fundlegacy, 0, len(fsIDs))}

	for _, fsID := range fsIDs {
		fundRec := fundMap[fsID]
		fund := models.Fundlegacy{
			FundID:   fundRec.FundID,
			FundName: fundRec.FundName,
		}

		// pick the value sub-map for this fund-series
		vm := valueMap[fsID]
		if vm == nil { // no data yet
			vm = make(map[string]string)
		}

		for _, m := range meta {
			/* skip if we already emitted this fieldNo */
			if len(fund.FundData) > 0 && fund.FundData[len(fund.FundData)-1].FieldNo == m.FieldNo {
				continue
			}

			g := fieldMap[m.FieldNo]

			/* 1. table lives at field level (14, 17, …) */
			if g.parent.IsTableData {
				fund.FundData = append(fund.FundData,
					buildTableFieldItemGroup(m.FieldNo, g, vm))
				continue
			}

			// 2. normal text / bool (options) block
			fund.FundData = append(fund.FundData,
				buildFieldItemGroup(g, vm))
		}
		resp.Funds = append(resp.Funds, fund)
	}

	// 3. sort funds by name
	sort.Sort(byFundName(resp.Funds))

	// 4. sort field_data inside every fund by field_no
	for i := range resp.Funds {
		sort.Sort(byFieldNo(resp.Funds[i].FundData))
	}

	return resp, nil
}

// table builder
func buildTableFieldItemGroup(fieldNo string, g *group, vm map[string]string) models.FieldItem {
	par := g.parent
	if par.FormLineItemNo == "" {
		for _, v := range g.children {
			if v.FormLineItemNo == fieldNo {
				par = v
				break
			}
		}
	}

	// Group children by row-key
	rowMap := make(map[string]map[string]string)
	for _, v := range g.children {
		if v.FormLineItemNo == fieldNo {
			continue
		}
		tokens := strings.SplitN(v.FormLineItemNo, "_", 3)
		if len(tokens) <= 2 {
			continue
		}
		rowKey, colKey := tokens[1], tokens[2]
		if rowMap[rowKey] == nil {
			rowMap[rowKey] = make(map[string]string)
		}
		rowMap[rowKey][colKey] = vm[v.FormLineItemNo]
	}

	// Stable row order
	rowKeys := make([]string, 0, len(rowMap))
	for k := range rowMap {
		rowKeys = append(rowKeys, k)
	}
	sort.Strings(rowKeys)

	// Convert to ordered maps
	tableData := make([]*OrderedMap, 0, len(rowMap))
	for _, rk := range rowKeys {
		var label string
		for _, v := range g.children {
			if strings.HasPrefix(v.FormLineItemNo, fieldNo+"_"+rk+"_") {
				label = v.LineText
				break
			}
		}

		// Create ordered map with s.name and s.text first
		obj := NewOrderedMap()
		obj.Set("s.name", rk)
		obj.Set("s.text", rk+" "+label)

		// Add remaining columns in sorted order
		colKeys := make([]string, 0, len(rowMap[rk]))
		for ck := range rowMap[rk] {
			colKeys = append(colKeys, ck)
		}
		sort.Strings(colKeys)

		for _, ck := range colKeys {
			obj.Set(ck, rowMap[rk][ck])
		}

		tableData = append(tableData, obj)
	}

	return models.FieldItem{
		FieldNo:          fieldNo,
		HasFieldItemText: par.LineText != "",
		FieldItemType:    par.DataType,
		HasTableData:     true,
		FieldText:        par.LineText,
		FieldHeading:     headingFor(fieldNo),
		Value:            vm[fieldNo],
		FieldData:        []models.LineDetail{},
		TableData:        tableData,
		Confirmed:        false,
		Editing:          true,
		IsMultipleAdd:    par.IsMultipleAdd,
		DisclaimerText:   par.DisclaimerText,
	}
}

// builder for text / bool
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
