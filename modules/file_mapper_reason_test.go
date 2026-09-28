package modules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"form_pf_backend/models"
)

func TestShortCodesFromFiles_UnmappedReason(t *testing.T) {
	path := writeMapperFile(t, "11111111-1111-1111-1111-111111111111"+marker+"notes.xlsx")
	funds := []models.FegFileIdentification{
		{ShortCode: "FSAE", FundName: "Fund Alpha", FileType: "Q1"},
	}

	mapped, unmapped := shortCodesFromFiles([]string{path}, funds)
	if len(mapped) != 0 {
		t.Fatalf("expected no fund candidates, got %#v", mapped)
	}
	if len(unmapped) != 1 {
		t.Fatalf("expected 1 unmapped file, got %d", len(unmapped))
	}
	if unmapped[0].Reason != unrecognizedFundReason {
		t.Fatalf("reason = %q", unmapped[0].Reason)
	}
	if unmapped[0].ShortCode != "" || unmapped[0].FundName != "" {
		t.Fatalf("unrecognized file should not be tied to a fund: %#v", unmapped[0])
	}
	if unmapped[0].FileName != "notes.xlsx" {
		t.Fatalf("filename should omit the storage prefix, got %q", unmapped[0].FileName)
	}
}

func TestDedupeStoredFiles_CollapsesUUIDPrefixCopies(t *testing.T) {
	dir := t.TempDir()
	org := "1. FSAAEM-FSAAEM~^investors20251231 capital register_palbreakout~.xls"
	older := filepath.Join(dir, "ee1fd554-cb6b-4201-b109-f4c9a0938d0f"+marker+org)
	newer := filepath.Join(dir, "4259c335-fc4d-4e21-b28d-8d87250cdb05"+marker+org)
	other := filepath.Join(dir, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"+marker+"2. FSAEM2-report.xlsx")
	for _, path := range []string{older, newer, other} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, past, past); err != nil {
		t.Fatal(err)
	}

	got := dedupeStoredFiles([]string{older, newer, other})
	if len(got) != 2 {
		t.Fatalf("expected 2 files after collapsing copies, got %#v", got)
	}
	if got[0] != newer {
		t.Fatalf("kept %q, want newest copy %q", got[0], newer)
	}
	if originalFileName(got[0]) != org {
		t.Fatalf("original name = %q", originalFileName(got[0]))
	}
}

func TestFindFileType_QuarterMatchHasNoReason(t *testing.T) {
	path := writeMapperFile(t, "11111111-1111-1111-1111-111111111111"+marker+"FSAE_2024-03.xlsx")
	funds := []models.FegFileIdentification{
		{ShortCode: "FSAE", FundName: "Fund Alpha", FileType: "Q1"},
		{ShortCode: "FSAE", FundName: "Fund Alpha", FileType: "Q2"},
	}

	mapped, unmapped := FindFileType(map[string][]string{"FSAE": {path}}, funds, "Private Investors")
	if len(unmapped) != 0 {
		t.Fatalf("expected file to map, unmapped=%#v", unmapped)
	}
	if len(mapped) != 1 {
		t.Fatalf("expected 1 mapping, got %#v", mapped)
	}
	if mapped[0].FileType != "Q1" || mapped[0].Reason != "" {
		t.Fatalf("mapped file = %#v", mapped[0])
	}
	if mapped[0].FileName != "FSAE_2024-03.xlsx" {
		t.Fatalf("filename = %q", mapped[0].FileName)
	}
}

func TestFindFileType_MissingQuarterReason(t *testing.T) {
	path := writeMapperFile(t, "11111111-1111-1111-1111-111111111111"+marker+"FSAE_misc.xlsx")
	funds := []models.FegFileIdentification{
		{ShortCode: "FSAE", FundName: "Fund Alpha", FileType: "Q1"},
		{ShortCode: "FSAE", FundName: "Fund Alpha", FileType: "Q2"},
	}

	mapped, unmapped := FindFileType(map[string][]string{"FSAE": {path}}, funds, "Private Investors")
	if len(mapped) != 0 {
		t.Fatalf("expected no mapping, got %#v", mapped)
	}
	if len(unmapped) != 1 {
		t.Fatalf("expected 1 unmapped file, got %d", len(unmapped))
	}
	got := unmapped[0]
	if got.ShortCode != "FSAE" || got.FundName != "Fund Alpha" {
		t.Fatalf("fund context = %#v", got)
	}
	if !strings.Contains(got.Reason, "Fund Alpha (FSAE)") || !strings.Contains(got.Reason, "reporting quarter") {
		t.Fatalf("reason = %q", got.Reason)
	}
	if strings.Contains(got.Reason, "stack") || strings.Contains(strings.ToLower(got.Reason), "error") {
		t.Fatalf("reason exposes internal detail: %q", got.Reason)
	}
}

func TestFindFileType_UnexpectedQuarterReason(t *testing.T) {
	path := writeMapperFile(t, "11111111-1111-1111-1111-111111111111"+marker+"FSAE_2024-09.xlsx")
	funds := []models.FegFileIdentification{
		{ShortCode: "FSAE", FundName: "Fund Alpha", FileType: "Q1"},
	}

	_, unmapped := FindFileType(map[string][]string{"FSAE": {path}}, funds, "Private Investors")
	if len(unmapped) != 1 {
		t.Fatalf("expected 1 unmapped file, got %#v", unmapped)
	}
	if !strings.Contains(unmapped[0].Reason, "Q3") || !strings.Contains(unmapped[0].Reason, "not expected") {
		t.Fatalf("reason = %q", unmapped[0].Reason)
	}
}

func TestFindFileType_FileTypeReason(t *testing.T) {
	path := writeMapperFile(t, "11111111-1111-1111-1111-111111111111"+marker+"Curio_One_misc.xlsx")
	period := "Q"
	funds := []models.FegFileIdentification{
		{ShortCode: "CUR", FundName: "Curio One", FileType: "capital_register", Period: &period},
		{ShortCode: "CUR", FundName: "Curio One", FileType: "nav_tb", Period: &period},
	}

	mapped, unmapped := FindFileType(map[string][]string{"CUR": {path}}, funds, "Curio")
	if len(mapped) != 0 {
		t.Fatalf("expected no mapping, got %#v", mapped)
	}
	if len(unmapped) != 1 {
		t.Fatalf("expected 1 unmapped file, got %#v", unmapped)
	}
	reason := unmapped[0].Reason
	if !strings.Contains(reason, "Curio One (CUR)") {
		t.Fatalf("reason = %q", reason)
	}
	if !strings.Contains(reason, "Capital Register") || !strings.Contains(reason, "Trial Balance") {
		t.Fatalf("reason = %q", reason)
	}
	if strings.Contains(reason, "capital_register") || strings.Contains(reason, "nav_tb") {
		t.Fatalf("reason exposes internal file type keys: %q", reason)
	}
}

func TestFindFileType_FileTypeStillMaps(t *testing.T) {
	path := writeMapperFile(t, "11111111-1111-1111-1111-111111111111"+marker+"Curio_One_capital_register.xlsx")
	period := "M"
	funds := []models.FegFileIdentification{
		{ShortCode: "CUR", FundName: "Curio One", FileType: "capital_register", Period: &period},
	}

	mapped, unmapped := FindFileType(map[string][]string{"CUR": {path}}, funds, "Curio")
	if len(unmapped) != 0 || len(mapped) != 1 {
		t.Fatalf("mapped=%#v unmapped=%#v", mapped, unmapped)
	}
	if mapped[0].FileType != "Capital Register" || mapped[0].Reason != "" {
		t.Fatalf("mapped file = %#v", mapped[0])
	}
}

func writeMapperFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
