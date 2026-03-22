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
)

var rootCmd = &cobra.Command{
	Use:   "tq",
	Short: "AI-powered task queue for Claude Code workers",
	Long: `Task queue backed by SQLite. Dispatch work to Claude Code workers via tmux.

Data model: project → task → action.
  - project: groups tasks, sets working directory
  - task: unit of work (status: open, review, done, blocked, archived)
  - action: dispatchable item linked to a prompt template (status: pending, running, done, failed, cancelled)

Typical flow: create a task, then create actions under it.
Pending actions are auto-dispatched by the queue worker (tq ui), or manually via tq dispatch.

All list commands output JSON.`,
	Example: `  # Quick start
  tq project create myapp ~/src/myapp
  tq task create "Implement feature X" --project 1
  tq action create review-pr --task 1 --title "Review PR #42"
  tq dispatch
  tq ui`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if database != nil {
			return nil
		}
		dir, err := configDir()
		if err != nil {
			return err
		}
		dbPath := filepath.Join(dir, "tq.db")
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
		if database != nil && !dbInjected {
			return database.Close()
		}
		return nil
	},
}

func configDir() (string, error) {
	if configDirOverride != "" {
		return configDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "tq")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return dir, nil
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(actionCmd)
	rootCmd.AddCommand(dispatchCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(projectCmd)
	rootCmd.AddCommand(promptCmd)
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

func resetFlagsRecursive(c *cobra.Command) {
	c.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
	for _, sub := range c.Commands() {
		resetFlagsRecursive(sub)
	}
}
