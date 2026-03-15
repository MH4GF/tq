package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:     "attach <action_id>",
	Short:   "Attach to a running action's tmux window",
	Example: `  tq action attach 3`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}
		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("get action: %w", err)
		}
		if !action.SessionID.Valid {
			return fmt.Errorf("action #%d has no tmux session info (action may not be running interactively)", id)
		}
		if os.Getenv("TMUX") == "" {
			return fmt.Errorf("this command must be run inside a tmux session")
		}
		return exec.Command("tmux", "select-window", "-t", fmt.Sprintf("%s:%s", action.SessionID.String, action.TmuxPane.String)).Run()
	},
}

func init() {
	actionCmd.AddCommand(attachCmd)
}
