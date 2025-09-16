package db

import (
	"form_pf_backend/models"
)

func FetchFormFields() ([]models.FormField, error) {
	var rows []models.FormField
	err := GetDB().Find(&rows).Error
	return rows, err
}

func FetchFormDataByFiling(filingID uint64) ([]models.FormDatum, error) {
	var rows []models.FormDatum
	err := GetDB().
		Where("filingld = ?", filingID).
		Find(&rows).
		Error
	return rows, err
}
