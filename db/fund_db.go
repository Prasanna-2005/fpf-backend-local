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

func GetFundMap(fundSeriesIDs []uint64) (map[uint64]models.Fund, error) {
	type result struct {
		FundSeriesid uint64 `gorm:"column:fundSeriesid"` // ← PK from join
		FundID       string `gorm:"column:fegFundId"`
		FundName     string `gorm:"column:fundName"`
	}

	var rows []result
	err := GetDB().
		Table("feg_funds").
		Select("DISTINCT form_data_legacy.fundSeriesid, feg_funds.fegFundId, feg_funds.fundName").
		Joins("JOIN form_data_legacy ON form_data_legacy.fundSeriesid = feg_funds.id").
		Where("form_data_legacy.fundSeriesid IN ?", fundSeriesIDs).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make(map[uint64]models.Fund, len(rows))
	for _, r := range rows {
		out[r.FundSeriesid] = models.Fund{
			FundID:   r.FundID,
			FundName: r.FundName,
		}
	}
	return out, nil
}
