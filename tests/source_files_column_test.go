package tests

import (
	"os"
	"path/filepath"
	"testing"

	"form_pf_backend/db"
	"form_pf_backend/models"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func TestSourceFilesColumnRoundTrip(t *testing.T) {
	loadBackendEnv(t)
	sqlDB := db.GetDB()

	var col string
	err := sqlDB.Raw(`
		SELECT COLUMN_NAME
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'form_data_legacy'
		  AND COLUMN_NAME = 'sourceFiles'`).Scan(&col).Error
	if err != nil {
		t.Fatalf("column lookup: %v", err)
	}
	if col != "sourceFiles" {
		t.Fatal("form_data_legacy.sourceFiles is missing")
	}

	id := uuid.New().String()
	var host models.FormDataLegacy
	if err := sqlDB.Select("filingId", "fundSeriesId", "fieldNo", "formLineNo", "createdBy").
		Where("isCurrentValue = ?", true).
		Limit(1).
		Find(&host).Error; err != nil {
		t.Fatalf("host row: %v", err)
	}
	if host.FilingID == "" {
		t.Skip("no form_data_legacy rows to attach a sourceFiles sample")
	}
	want := `["tb.xlsx","unfunded.xlsx"]`
	row := models.FormDataLegacy{
		ID:             id,
		FilingID:       host.FilingID,
		FieldNo:        host.FieldNo,
		FundSeriesid:   host.FundSeriesid,
		FormLineNo:     host.FormLineNo,
		Value:          "src-test-100",
		SourceFiles:    want,
		CreatedBy:      host.CreatedBy,
		UpdatedBy:      host.CreatedBy,
		IsCurrentValue: false,
	}
	if err := sqlDB.Create(&row).Error; err != nil {
		t.Fatalf("insert: %v", err)
	}
	t.Cleanup(func() {
		sqlDB.Where("id = ?", id).Delete(&models.FormDataLegacy{})
	})

	var got models.FormDataLegacy
	if err := sqlDB.Select("id", "sourceFiles", "value").Where("id = ?", id).First(&got).Error; err != nil {
		t.Fatalf("select: %v", err)
	}
	if got.SourceFiles != want {
		t.Fatalf("sourceFiles = %q, want %q", got.SourceFiles, want)
	}
	files, err := models.ParseSourceFiles(got.SourceFiles)
	if err != nil {
		t.Fatalf("parse sourceFiles: %v", err)
	}
	if len(files) != 2 || files[0] != "tb.xlsx" || files[1] != "unfunded.xlsx" {
		t.Fatalf("parsed %v", files)
	}

	history, err := db.GetFieldHistoryDB(host.FilingID, host.FundSeriesid, host.FieldNo)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	found := false
	for _, h := range history {
		if h.Value == "src-test-100" {
			found = true
			if h.SourceFiles != want {
				t.Fatalf("history sourceFiles = %q", h.SourceFiles)
			}
		}
	}
	if !found {
		t.Fatal("expected history row for the inserted sourceFiles sample")
	}
}

func loadBackendEnv(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{wd, filepath.Join(wd, "..")} {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				t.Fatalf("load %s: %v", path, err)
			}
			return
		}
	}
	t.Fatal(".env not found")
}
