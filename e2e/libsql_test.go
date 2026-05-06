//go:build libsql_e2e

// Replays the testscript scenarios under e2e/testdata/script against a
// live libsql endpoint (sqld container or Turso) to verify cross-driver
// compatibility. Build tag `libsql_e2e` keeps it out of the default
// suite. Run with:
//
//	export TQ_DB_URL="libsql://localhost:8080?tls=0"   # sqld container
//	# or:
//	export TQ_DB_URL="libsql://your-db.turso.io?authToken=..."
//	go test -tags libsql_e2e ./e2e/ -run TestLibsqlE2E -v -timeout 10m
//
// Scenarios share one DB and run sequentially. The runner drops all tq
// tables and re-applies migrations before each scenario so each one
// starts from autoincrement id=1.
package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/MH4GF/tq/db"
)

// libsqlSkipScenarios names .txtar files that don't make sense on a
// remote DB. db_precedence is specifically about local file resolution.
var libsqlSkipScenarios = map[string]bool{
	"db_precedence": true,
}

func TestLibsqlE2E(t *testing.T) {
	url := os.Getenv("TQ_DB_URL")
	if url == "" {
		t.Skip("TQ_DB_URL not set; skipping libsql e2e")
	}

	scenarios, err := filepath.Glob("testdata/script/*.txtar")
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range scenarios {
		name := strings.TrimSuffix(filepath.Base(path), ".txtar")
		if libsqlSkipScenarios[name] {
			continue
		}
		t.Run(name, func(t *testing.T) {
			resetLibsqlSchema(t, url)

			testscript.Run(t, testscript.Params{
				Files: []string{path},
				Setup: func(env *testscript.Env) error {
					// Hand the libsql URL straight through to the child tq
					// process. The token (if any) lives inside the URL.
					env.Setenv("TQ_DB_URL", url)
					env.Setenv("HOME", env.WorkDir)

					tmuxDir, err := os.MkdirTemp("/tmp", "tq-libsql-e2e-tmux-")
					if err != nil {
						return err
					}
					env.Setenv("TMUX_TMPDIR", tmuxDir)
					env.Defer(func() { _ = os.RemoveAll(tmuxDir) })
					return nil
				},
			})
		})
	}
}

// resetLibsqlSchema drops every tq table on the shared libsql DB and
// reapplies the migration so the next scenario starts with a clean
// slate (autoincrement id=1, no leftover rows).
func resetLibsqlSchema(t *testing.T, url string) {
	t.Helper()
	d, err := db.Open(url)
	if err != nil {
		t.Fatalf("open libsql: %v", err)
	}
	defer func() { _ = d.Close() }()

	tables := []string{"events", "worker_heartbeats", "schedules", "actions", "tasks", "projects"}
	for _, tbl := range tables {
		if _, err := d.Exec("DROP TABLE IF EXISTS " + tbl); err != nil {
			t.Fatalf("drop %s: %v", tbl, err)
		}
	}
	if err := d.Migrate(); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
}
