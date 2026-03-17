package db_test

import (
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
)

func TestFormatLocal(t *testing.T) {
	// Determine expected local offset
	utcStr := "2026-03-17 03:00:00"
	parsed, _ := time.Parse("2006-01-02 15:04:05", utcStr)
	want := parsed.Local().Format("2006-01-02 15:04:05")

	got := db.FormatLocal(utcStr)
	if got != want {
		t.Errorf("FormatLocal(%q) = %q, want %q", utcStr, got, want)
	}
}

func TestFormatLocal_InvalidFallback(t *testing.T) {
	invalid := "not-a-timestamp"
	got := db.FormatLocal(invalid)
	if got != invalid {
		t.Errorf("FormatLocal(%q) = %q, want fallback %q", invalid, got, invalid)
	}
}
