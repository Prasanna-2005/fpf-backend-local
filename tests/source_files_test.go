package tests

import (
	"encoding/json"
	"testing"

	"form_pf_backend/models"
)

func TestParseSourceFiles(t *testing.T) {
	got := models.ParseSourceFiles(`["tb.xlsx","unfunded.xlsx"]`)
	if len(got) != 2 || got[0] != "tb.xlsx" || got[1] != "unfunded.xlsx" {
		t.Fatalf("got %v", got)
	}
	if len(models.ParseSourceFiles("")) != 0 {
		t.Fatal("empty raw should be empty slice")
	}
	if len(models.ParseSourceFiles("not-json")) != 0 {
		t.Fatal("invalid json should be empty slice")
	}
	raw, err := json.Marshal([]string{"a.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	got = models.ParseSourceFiles(string(raw))
	if len(got) != 1 || got[0] != "a.xlsx" {
		t.Fatalf("got %v", got)
	}
}
