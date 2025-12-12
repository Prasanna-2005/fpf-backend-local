package modules

import (
	"fmt"
	"form_pf_backend/models"
	"sort"
)

// byFundName implements sort.Interface for []models.Fundlegacy
type byFundName []models.Fundlegacy

func (s byFundName) Len() int           { return len(s) }
func (s byFundName) Less(i, j int) bool { return s[i].FundName < s[j].FundName }
func (s byFundName) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// byFieldNo implements sort.Interface for []models.FieldItem
type byFieldNo []models.FieldItem

func (s byFieldNo) Len() int      { return len(s) }
func (s byFieldNo) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s byFieldNo) Less(i, j int) bool {
	// extract leading digits only
	var ni, nj int
	fmt.Sscanf(s[i].FieldNo, "%d", &ni)
	fmt.Sscanf(s[j].FieldNo, "%d", &nj)
	return ni < nj
}

// numericPrefix extracts the leading digits from a line_item_no like "7_a"
func numericPrefix(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// sortFieldData sorts the FieldData slice inside every FieldItem
func sortFieldData(fd []models.LineDetail) {
	sort.Slice(fd, func(i, j int) bool {
		return numericPrefix(fd[i].LineItemNo) < numericPrefix(fd[j].LineItemNo)
	})
}
