package db

import (
	"form_pf_backend/models"
)

func GetAllFunds() ([]models.Fund, error) {
	var funds []models.Fund
	if err := sqlDb.Find(&funds).Error; err != nil {
		return nil, err
	}
	return funds, nil
}

func GetFundMap(fundSeriesIDs []string) (map[string]models.Fund, error) {
	type result struct {
		FundSeriesid string `gorm:"column:fundSeriesid"` // ← PK from join
		FundID       string `gorm:"column:fegFundId"`
		FundName     string `gorm:"column:fundName"`
		FundType     string `gorm:"column:fundType"`
	}

	var rows []result
	err := GetDB().
		Table("feg_funds").
		Select("DISTINCT form_data_legacy.fundSeriesid, feg_funds.fegFundId, feg_funds.fundName,feg_funds.fundType").
		Joins("JOIN form_data_legacy ON form_data_legacy.fundSeriesid = feg_funds.fegFundId").
		Where("form_data_legacy.fundSeriesid IN ?", fundSeriesIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.Fund, len(rows))
	for _, r := range rows {
		out[r.FundSeriesid] = models.Fund{
			FundID:   r.FundID,
			FundName: r.FundName,
			FundType: r.FundType,
		}
	}

	return out, nil
}

func GetCompLogs(series, clientCode string) ([]models.CompLogs, error) {
	var compLogs []models.CompLogs
	if err := sqlDb.Where("series = ? AND clientCode = ?", series, clientCode).Find(&compLogs).Error; err != nil {
		return nil, err
	}
	return compLogs, nil
}
