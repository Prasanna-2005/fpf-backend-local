package db

import (
	"testing"
	"time"

	"form_pf_backend/models"
)

func TestFilesPresentAtKeepsUnusedFilesFromThatVersion(t *testing.T) {
	submitted := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	replaced := time.Date(2026, 1, 2, 14, 0, 0, 0, time.UTC)
	extracted := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	rows := []models.FileMapper{
		{FileName: "old-tb.xlsx", FilingPath: "/a/old-tb.xlsx", FundShortCode: "FSAE", Quarter: "nav_tb", CreatedAt: submitted},
		{FileName: "new-tb.xlsx", FilingPath: "/a/new-tb.xlsx", FundShortCode: "FSAE", Quarter: "nav_tb", CreatedAt: replaced},
		{FileName: "liq.xlsx", FilingPath: "/a/liq.xlsx", FundShortCode: "SHARED", Quarter: "NA", CreatedAt: submitted},
		{FileName: "cap.xlsx", FilingPath: "/a/cap.xlsx", FundShortCode: "FSAE", Quarter: "capital_register", CreatedAt: submitted.Add(5 * time.Minute)},
	}

	got := filesPresentAt(rows, extracted)
	byKey := map[string]string{}
	for _, row := range got {
		byKey[slotKey(row)] = row.FileName
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 files, got %#v", byKey)
	}
	if byKey["FSAE|nav_tb|"] != "old-tb.xlsx" {
		t.Fatalf("trial balance slot = %#v", byKey)
	}
	if byKey["FSAE|capital_register|"] != "cap.xlsx" {
		t.Fatalf("unused capital register missing: %#v", byKey)
	}
	if byKey["SHARED|NA|"] != "liq.xlsx" {
		t.Fatalf("shared file missing: %#v", byKey)
	}
}

func TestFilesPresentAtUsesReplacementAfterNextExtraction(t *testing.T) {
	first := time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC)
	second := time.Date(2026, 1, 2, 14, 0, 0, 0, time.UTC)
	rows := []models.FileMapper{
		{FileName: "old-tb.xlsx", FilingPath: "/a/old-tb.xlsx", FundShortCode: "FSAE", Quarter: "nav_tb", CreatedAt: first},
		{FileName: "new-tb.xlsx", FilingPath: "/a/new-tb.xlsx", FundShortCode: "FSAE", Quarter: "nav_tb", CreatedAt: second},
		{FileName: "cap.xlsx", FilingPath: "/a/cap.xlsx", FundShortCode: "FSAE", Quarter: "capital_register", CreatedAt: first},
	}

	got := filesPresentAt(rows, time.Date(2026, 1, 2, 16, 0, 0, 0, time.UTC))
	byKey := map[string]string{}
	for _, row := range got {
		byKey[slotKey(row)] = row.FileName
	}
	if byKey["FSAE|nav_tb|"] != "new-tb.xlsx" || byKey["FSAE|capital_register|"] != "cap.xlsx" {
		t.Fatalf("version 2 slots = %#v", byKey)
	}
}
