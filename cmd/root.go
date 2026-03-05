package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/MH4GF/tq/db"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	flagDir       string
	database      *db.DB
	tqDirResolved string
	dbInjected    bool
)

var rootCmd = &cobra.Command{
	Use:   "tq",
	Short: "Task Queue CLI",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if database != nil {
			return nil
		}
		dir := resolveTQDir()
		if dir == "" {
			return fmt.Errorf("cannot resolve TQ_DIR: use --dir, TQ_DIR env, or run inside a git repo")
		}
		tqDirResolved = dir
		dbPath := filepath.Join(dir, "tq.db")
		var err error
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

func resolveTQDir() string {
	if flagDir != "" {
		return flagDir
	}
	if dir := os.Getenv("TQ_DIR"); dir != "" {
		return dir
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return filepath.Join(strings.TrimSpace(string(out)), "tq")
	}
	return ""
}

func init() {
	rootCmd.Version = buildVersion()
	rootCmd.PersistentFlags().StringVar(&flagDir, "dir", "", "TQ directory path")
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

func SetTQDir(dir string) {
	tqDirResolved = dir
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
