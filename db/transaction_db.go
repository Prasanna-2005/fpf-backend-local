package db

import (
	"form_pf_backend/db"
	"form_pf_backend/models"
)

func FetchFormFields() ([]models.FormField, error) {
	var rows []models.FormField
	err := db.GetDB().Find(&rows).Error
	return rows, err
}

func FetchFormDataByFiling(filingID uint64) ([]models.FormDatum, error) {
	var rows []models.FormDatum
	err := db.GetDB().
		Where("filingld = ?", filingID). // typo matches your DB column
		Find(&rows).
		Error
	return rows, err
}
