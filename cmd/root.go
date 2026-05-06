package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/MH4GF/tq/db"
)

var (
	database          db.Store
	dbInjected        bool
	configDirOverride string
	dbPathFlag        string
)

var rootCmd = &cobra.Command{
	Use:   "tq",
	Short: "AI-powered task queue for Claude Code workers",
	Long: `Task queue backed by SQLite. Dispatch work to Claude Code workers via tmux.

Data model: project → task → action.
  - project: groups tasks, sets working directory
  - task: unit of work (status: open, done, archived)
  - action: dispatchable unit of work with an instruction (status: pending, running, dispatched, done, failed, cancelled)

Typical flow: create a task, then create actions under it.
Pending actions are auto-dispatched by the queue worker (tq ui), or manually via tq action dispatch.

Database location precedence:
  1. --db flag
  2. TQ_DB_URL environment variable
  3. ~/.config/tq/tq.db (default)

The --db flag and TQ_DB_URL accept either a local sqlite file path
or a libsql URL (libsql://host?authToken=...). For Turso/self-hosted
sqld endpoints, embed the auth token in the URL query string.

All list commands output JSON.`,
	Example: `  # Quick start
  tq project create myapp ~/src/myapp
  tq task create "Implement feature X" --project 1
  tq action create review-pr --task 1 --title "Review PR #42"
  tq search "feature X"
  tq action dispatch 1
  tq ui`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if database != nil {
			return nil
		}
		dbPath, err := resolveDBPath()
		if err != nil {
			return err
		}
		if err := ensureLocalDBDir(dbPath); err != nil {
			return err
		}
		database, err = db.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		if err := database.Migrate(); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return closeDBIfOwned()
	},
}

func resolveDBPath() (string, error) {
	if dbPathFlag != "" {
		return dbPathFlag, nil
	}
	if p := os.Getenv("TQ_DB_URL"); p != "" {
		return p, nil
	}
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tq.db"), nil
}

// ensureLocalDBDir creates the parent directory for a local sqlite DB.
// No-op for libsql URLs.
func ensureLocalDBDir(dbPath string) error {
	if db.IsLibsqlURL(dbPath) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("create db dir: %w", err)
	}
	return nil
}

func configDir() (string, error) {
	if configDirOverride != "" {
		return configDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	return filepath.Join(home, ".config", "tq"), nil
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVar(&dbPathFlag, "db", "",
		"DB path or libsql URL (overrides TQ_DB_URL; default: ~/.config/tq/tq.db)")
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(actionCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(scheduleCmd)
	rootCmd.AddCommand(searchCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func GetRootCmd() *cobra.Command {
	return rootCmd
}

func SetDB(d db.Store) {
	database = d
	dbInjected = true
}

func SetConfigDir(dir string) {
	configDirOverride = dir
}

func ResetForTest() {
	resetFlagsRecursive(rootCmd)
}

// ResetState resets all package-level state. Required because testscript
// re-enters Execute() multiple times within a single Go process; without
// this, the previous run's database handle and flag values leak across
// scenarios.
func ResetState() {
	_ = closeDBIfOwned()
	database = nil
	dbInjected = false
	configDirOverride = ""
	dbPathFlag = ""
	resetFlagsRecursive(rootCmd)
}

// closeDBIfOwned closes the package-level database only if it was opened
// by Execute() (not injected via SetDB). Nils the handle on success so
// the next Execute() reopens cleanly.
func closeDBIfOwned() error {
	if database == nil || dbInjected {
		return nil
	}
	err := database.Close()
	database = nil
	return err
}

func resetFlagsRecursive(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range c.Commands() {
		resetFlagsRecursive(sub)
	}
}
