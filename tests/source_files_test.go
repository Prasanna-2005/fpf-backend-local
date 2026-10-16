package tests

import (
	"encoding/json"
	"testing"

	"form_pf_backend/models"
)

func TestParseSourceFiles(t *testing.T) {
	got, err := models.ParseSourceFiles(`["tb.xlsx","unfunded.xlsx"]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "tb.xlsx" || got[1] != "unfunded.xlsx" {
		t.Fatalf("got %v", got)
	}
	empty, err := models.ParseSourceFiles("")
	if err != nil || len(empty) != 0 {
		t.Fatal("empty raw should be empty slice")
	}
	invalid, err := models.ParseSourceFiles("not-json")
	if err == nil || len(invalid) != 0 {
		t.Fatal("invalid json should be empty slice and an error")
	}
	raw, err := json.Marshal([]string{"a.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	got, err = models.ParseSourceFiles(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "a.xlsx" {
		t.Fatalf("got %v", got)
	}
}
