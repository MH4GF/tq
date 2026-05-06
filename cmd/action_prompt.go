package cmd

import (
	"encoding/json"
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
under the macOS pty MAX_CANON limit regardless of instruction length.

Output is byte-identical to dispatch.RenderPrompt for the same action,
ending with exactly one trailing LF.`,
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
		meta := map[string]any{}
		if action.Metadata != "" && action.Metadata != "{}" {
			if err := json.Unmarshal([]byte(action.Metadata), &meta); err != nil {
				return fmt.Errorf("parse metadata: %w", err)
			}
		}
		instruction, _ := meta[dispatch.MetaKeyInstruction].(string)
		if strings.TrimSpace(instruction) == "" {
			return fmt.Errorf("action #%d has empty instruction", id)
		}
		mode, _ := meta[dispatch.MetaKeyMode].(string)
		if mode == "" {
			mode = dispatch.ModeInteractive
		}
		// TODO: drop the isResume parameter from RenderPrompt in a follow-up
		// PR. The interactive path hardcodes false here because threading a
		// dedicated --resume flag (or ActionConfig.IsResume) is not worth the
		// single claude turn it would save by suppressing the "Required first
		// step" postamble for resumed sessions. Once noninteractive/remote
		// callers also stop relying on the suppression, the parameter goes
		// away entirely.
		out := dispatch.RenderPrompt(instruction, action.ID, action.TaskID, mode, false)
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
