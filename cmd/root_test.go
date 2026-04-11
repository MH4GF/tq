package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDBPath(t *testing.T) {
	t.Run("flag takes precedence over env and default", func(t *testing.T) {
		t.Setenv("TQ_DB_PATH", "/env/path/tq.db")
		prev := dbPathFlag
		dbPathFlag = "/flag/path/tq.db"
		t.Cleanup(func() { dbPathFlag = prev })

		got, err := resolveDBPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/flag/path/tq.db" {
			t.Errorf("path = %q, want %q", got, "/flag/path/tq.db")
		}
	})

	t.Run("env used when flag empty", func(t *testing.T) {
		t.Setenv("TQ_DB_PATH", "/env/path/tq.db")
		prev := dbPathFlag
		dbPathFlag = ""
		t.Cleanup(func() { dbPathFlag = prev })

		got, err := resolveDBPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/env/path/tq.db" {
			t.Errorf("path = %q, want %q", got, "/env/path/tq.db")
		}
	})

	t.Run("default uses configDir", func(t *testing.T) {
		t.Setenv("TQ_DB_PATH", "")
		prev := dbPathFlag
		dbPathFlag = ""
		t.Cleanup(func() { dbPathFlag = prev })

		dir := t.TempDir()
		prevDir := configDirOverride
		configDirOverride = dir
		t.Cleanup(func() { configDirOverride = prevDir })

		got, err := resolveDBPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := filepath.Join(dir, "tq.db")
		if got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
	})
}

func TestDBFlag_EndToEnd(t *testing.T) {
	t.Setenv("TQ_DB_PATH", "")

	prevDB, prevInjected := database, dbInjected
	database = nil
	dbInjected = false
	t.Cleanup(func() {
		if database != nil && !dbInjected {
			_ = database.Close()
		}
		database = prevDB
		dbInjected = prevInjected
	})

	ResetForTest()

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "nested", "demo.db")
	workDir := filepath.Join(tmp, "workdir")

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--db", dbPath, "project", "create", "demo", workDir})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created at %q: %v", dbPath, err)
	}
}
