package modules

import (
	"fmt"
	"form_pf_backend/db"
	"form_pf_backend/models"
	"sort"
	"strconv"
	"strings"
)

type compGroup struct {
	parent   models.FormFieldMSLegacy
	children []models.FormFieldMSLegacy
}

// computation entry point
func BuildComp(filingID, series, clientCode string) (*models.ComputationLogsLegacy, error) {
	meta, err := db.FetchFormFieldMSLegacy()
	if err != nil {
		return nil, err
	}

	fsIDs, err := db.FetchFundSeriesForFiling(filingID)
	if err != nil {
		return nil, err
	}
	if len(fsIDs) == 0 {
		return &models.ComputationLogsLegacy{Funds: []models.FundlegacyComputation{}},
			fmt.Errorf("extraction is still in process")
	}

	// valueMap is now 2-level: fundSeriesid -> formLineNo -> value
	valueMap, err := db.GetComputationlogs(series, clientCode)
	if err != nil {
		return nil, err
	}

	fundMap, err := db.GetFundMap(fsIDs)
	if err != nil {
		return nil, err
	}

	// / 1. group meta rows by fieldNo
	fieldMap := make(map[string]*compGroup)
	for _, m := range meta {
		if m.FormLineItemNo == m.FieldNo {
			if g := fieldMap[m.FieldNo]; g == nil {
				fieldMap[m.FieldNo] = &compGroup{parent: m}
			} else {
				g.parent = m
			}
			continue
		}
		if fieldMap[m.FieldNo] == nil {
			fieldMap[m.FieldNo] = &compGroup{}
		}
		fieldMap[m.FieldNo].children = append(fieldMap[m.FieldNo].children, m)
	}

	// 2. build response
	computationResp := &models.ComputationLogsLegacy{Funds: make([]models.FundlegacyComputation, 0, len(fsIDs))}

	for _, fsID := range fsIDs {
		fundRec := fundMap[fsID]
		fund := models.FundlegacyComputation{
			FundID:   fundRec.FundID,
			FundName: fundRec.FundName,
		}

		// pick the value sub-map for this fund-series
		vm := valueMap[fsID]
		if vm == nil { // no data yet
			vm = make(map[string][2]string)
		}
		seen := make(map[string]bool)
		for _, m := range meta {
			/* skip if we already emitted this fieldNo */
			fieldNoInt, _ := strconv.Atoi(m.FieldNo)
			if fieldNoInt > 17 && fundRec.FundType != "HEDGE" {
				continue
			}
			if seen[m.FieldNo] {
				continue
			}
			seen[m.FieldNo] = true

			g := fieldMap[m.FieldNo]

			if g.parent.IsTableData {
				fund.FundComp = append(fund.FundComp,
					buildTablelogs(m.FieldNo, g, vm))
				continue
			}
			if g.parent.IsMCQ {
				fund.FundComp = append(fund.FundComp, buildTextLogs(g, vm))
				continue
			}

			// 2. normal text / bool (options) block
			fund.FundComp = append(fund.FundComp,
				buildTextLogs(g, vm))
		}
		computationResp.Funds = append(computationResp.Funds, fund)
	}

	// 3. sort funds by name
	sort.Sort(byFundNameComp(computationResp.Funds))

	// 4. sort field_data inside every fund by field_no
	for i := range computationResp.Funds {
		sort.Sort(byFieldComp(computationResp.Funds[i].FundComp))
	}

	return computationResp, nil
}

// table builder
func buildTablelogs(fieldNo string, g *compGroup, vm map[string][2]string) models.FieldComputation {
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
	computationTypeMap := make(map[string]map[string]string)
	rowMap["metadata"] = make(map[string]string)

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
			computationTypeMap[rowKey] = make(map[string]string)
		}
		rowMap[rowKey][colKey] = vm[v.FormLineItemNo][1]
		rowMap["metadata"][colKey] = v.DataType
		computationTypeMap[rowKey][colKey] = vm[v.FormLineItemNo][0]

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
		if rk == "metadata" {
			continue
		}

		// Create ordered map with s.name and s.text first
		obj := NewOrderedMap()
		obj.Set("s.name", rk)
		obj.Set("s.computationType", computationTypeMap[rk])

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
	return models.FieldComputation{
		FieldNo:       fieldNo,
		FieldItemType: par.DataType,
		HasTableData:  true,
		TableMetadata: rowMap["metadata"],
		Type:          vm[fieldNo][0],
		Formula:       vm[fieldNo][1],
		FieldData:     []models.LineComputation{},
		TableData:     tableData,
	}
}

// builder for text / bool
func buildTextLogs(g *compGroup, vm map[string][2]string) models.FieldComputation {
	par := g.parent
	fItem := models.FieldComputation{
		FieldNo:          par.FieldNo,
		FieldItemType:    par.DataType,
		HasFieldItemText: par.LineText != "",
		FieldText:        par.LineText,
		HasTableData:     par.IsTableData,
		Type:             vm[par.FormLineItemNo][0],
		Formula:          vm[par.FormLineItemNo][1],
	}

	fItem.FieldData = make([]models.LineComputation, 0, len(g.children))
	for _, l := range g.children {
		fItem.FieldData = append(fItem.FieldData, models.LineComputation{
			LineItemNo:   l.FormLineItemNo,
			LineItemType: l.DataType,
			LineText:     l.LineText,
			Type:         vm[l.FormLineItemNo][0],
			Formula:      vm[l.FormLineItemNo][1],
			TableData:    nil,
		})
	}
	return fItem
}
