package db_test

import (
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestOpen(t *testing.T) {
	d := testutil.NewTestDB(t)
	var mode string
	if err := d.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" && mode != "memory" {
		t.Errorf("expected wal or memory journal mode, got %s", mode)
	}
}

func TestMigrate(t *testing.T) {
	d := testutil.NewTestDB(t)

	tables := []string{"projects", "tasks", "actions"}
	for _, table := range tables {
		var name string
		err := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestSeedTestProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3 projects, got %d", count)
	}
}

func TestMigrate_FixOpenActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "test", "", "{}", "")
	d.InsertAction("test", "test", taskID, "{}", "pending")
	// Simulate legacy "open" status that slipped in before validation was added
	d.Exec("UPDATE actions SET status = 'open' WHERE id = 1")

	// Re-run migration to fix the stuck action
	if err := d.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	a, err := d.GetAction(1)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if a.Status != "cancelled" {
		t.Errorf("status = %q, want %q", a.Status, "cancelled")
	}
	if !a.CompletedAt.Valid {
		t.Error("expected completed_at to be set")
	}
}

func TestClose(t *testing.T) {
	d := testutil.NewTestDB(t)
	if err := d.Close(); err != nil {
		t.Errorf("unexpected error on close: %v", err)
	}
}
