package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/dispatch"
)

var actionPromptCmd = &cobra.Command{
	Use:   "prompt <id>",
	Short: "Render the wrapped claude prompt for an action",
	Long: `Render the wrapped claude prompt (instruction + tq action context postamble)
for an action and write it to stdout. Used by interactive dispatch as
` + "`claude \"$(tq action prompt <id>)\"`" + ` so the tmux send-keys payload stays
under the macOS pty MAX_CANON limit regardless of instruction length.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("get action: %w", err)
		}
		meta, err := dispatch.ParseActionMetadata(action.Metadata)
		if err != nil {
			return fmt.Errorf("parse metadata: %w", err)
		}
		instruction, _ := meta[dispatch.MetaKeyInstruction].(string)
		if strings.TrimSpace(instruction) == "" {
			return fmt.Errorf("action #%d has empty instruction", id)
		}
		mode, _ := meta[dispatch.MetaKeyMode].(string)
		if mode == "" {
			mode = dispatch.ModeInteractive
		}
		out := dispatch.RenderPrompt(instruction, action.ID, action.TaskID, mode)
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		_, err = io.WriteString(cmd.OutOrStdout(), out)
		return err
	},
}

func init() {
	actionCmd.AddCommand(actionPromptCmd)
}
