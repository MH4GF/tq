package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDBPath(t *testing.T) {
	configDir := t.TempDir()

	tests := []struct {
		name      string
		env       string
		flag      string
		configDir string
		want      string
	}{
		{
			name: "flag takes precedence over env and default",
			env:  "/env/path/tq.db",
			flag: "/flag/path/tq.db",
			want: "/flag/path/tq.db",
		},
		{
			name: "env used when flag empty",
			env:  "/env/path/tq.db",
			want: "/env/path/tq.db",
		},
		{
			name:      "default uses configDir",
			configDir: configDir,
			want:      filepath.Join(configDir, "tq.db"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("TQ_DB_PATH", tt.env)
			prev := dbPathFlag
			dbPathFlag = tt.flag
			t.Cleanup(func() { dbPathFlag = prev })

			if tt.configDir != "" {
				prevDir := configDirOverride
				configDirOverride = tt.configDir
				t.Cleanup(func() { configDirOverride = prevDir })
			}

			got, err := resolveDBPath()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("path = %q, want %q", got, tt.want)
			}
		})
	}
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
