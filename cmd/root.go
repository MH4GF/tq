package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MH4GF/tq/db"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	database         *db.DB
	dbInjected       bool
	configDirOverride string
)

var rootCmd = &cobra.Command{
	Use:   "tq",
	Short: "Task Queue CLI",
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
	rootCmd.Version = buildVersion()
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(actionCmd)
	dispatchCmd.Hidden = true
	classifyCmd.Hidden = true
	watchCmd.Hidden = true
	runCmd.Hidden = true
	rootCmd.AddCommand(dispatchCmd)
	rootCmd.AddCommand(classifyCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(uiCmd)
	rootCmd.AddCommand(projectCmd)
}

func Execute() error {
	return rootCmd.Execute()
}

func GetRootCmd() *cobra.Command {
	return rootCmd
}

func SetDB(d *db.DB) {
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
