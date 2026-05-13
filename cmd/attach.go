package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/dispatch"
)

var attachCmd = &cobra.Command{
	Use:   "attach <action_id>",
	Short: "Attach to a running action (tmux window or claude agent view)",
	Example: `  tq action attach 3
  # experimental_bg actions open via 'claude attach <short>'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}
		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("get action: %w", err)
		}

		meta, err := dispatch.ParseActionMetadata(action.Metadata)
		if err != nil {
			return fmt.Errorf("parse action metadata: %w", err)
		}
		if mode, _ := meta[dispatch.MetaKeyMode].(string); mode == dispatch.ModeBg {
			short, _ := meta[dispatch.MetaKeyDaemonShort].(string)
			if short == "" {
				return fmt.Errorf("action #%d has no daemon_short yet (bg dispatch may still be in flight)", id)
			}
			attachCmd := exec.Command("claude", "attach", short)
			attachCmd.Stdin = os.Stdin
			attachCmd.Stdout = os.Stdout
			attachCmd.Stderr = os.Stderr
			return attachCmd.Run()
		}

		if !action.TmuxSession.Valid || !action.TmuxWindow.Valid {
			return fmt.Errorf("action #%d has no tmux session info (action may not be running interactively)", id)
		}
		if os.Getenv("TMUX") == "" {
			return fmt.Errorf("this command must be run inside a tmux session")
		}
		return exec.Command("tmux", "select-window", "-t", fmt.Sprintf("%s:%s", action.TmuxSession.String, action.TmuxWindow.String)).Run()
	},
}

func init() {
	actionCmd.AddCommand(attachCmd)
}
