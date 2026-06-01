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
	Short: "Attach to a running action (claude agent view)",
	Example: `  tq action attach 3
  # all local actions open via 'claude attach <daemon_short>'`,
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
		if mode, _ := meta[dispatch.MetaKeyMode].(string); mode == dispatch.ModeRemote {
			return fmt.Errorf("action #%d uses remote mode; attach is not supported", id)
		}
		short, _ := meta[dispatch.MetaKeyDaemonShort].(string)
		if short == "" {
			return fmt.Errorf("action #%d has no daemon_short yet (dispatch may still be in flight)", id)
		}
		attach := exec.Command("claude", "attach", short)
		attach.Stdin = os.Stdin
		attach.Stdout = os.Stdout
		attach.Stderr = os.Stderr
		return attach.Run()
	},
}

func init() {
	actionCmd.AddCommand(attachCmd)
}
