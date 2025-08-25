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
