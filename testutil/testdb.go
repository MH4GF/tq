package testutil

import (
	"testing"

	"github.com/MH4GF/tq/db"
)

func NewTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func SeedTestProjects(t *testing.T, d *db.DB) {
	t.Helper()
	projects := []struct{ name, workDir, metadata string }{
		{"immedio", "~/ghq/github.com/immedioinc/immedio", `{"gh_owner":"immedioinc","repos":["immedioinc/immedio"]}`},
		{"hearable", "~/ghq/github.com/thehearableapp/hearable-app", `{"gh_owner":"thehearableapp","repos":["thehearableapp/hearable-app","thehearableapp/hearable-survey"]}`},
		{"works", "~/ghq/github.com/MH4GF/works", `{"gh_owner":"MH4GF","repos":["MH4GF/works"]}`},
	}
	for _, p := range projects {
		if _, err := d.InsertProject(p.name, p.workDir, p.metadata); err != nil {
			t.Fatal(err)
		}
	}
}
