package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
)

var (
	resumeMessage string
	resumeMode    string
	resumeSession string
)

var actionResumeCmd = &cobra.Command{
	Use:   "resume <action_id>",
	Short: "Resume the claude session of a closed action",
	Long: `Create a new action that resumes the claude session of a previously
completed/failed/cancelled action via 'claude --resume <session_id>'.

The new action inherits only the parent's claude_session_id; other claude_args
(--worktree, --permission-mode, etc.) are NOT inherited because the resumed
claude session restores its own context.`,
	Example: `  tq action resume 42
  tq action resume 42 --message "前回失敗したので別アプローチで"
  tq action resume 42 --mode noninteractive`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		parentID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID %q: %w", args[0], err)
		}

		newID, err := database.ResumeAction(parentID, db.ResumeOptions{
			Message: resumeMessage,
			Mode:    resumeMode,
		})
		if err != nil {
			return err
		}

		action, err := database.ClaimPending(ctx, newID)
		if err != nil {
			return fmt.Errorf("claim resume action #%d: %w", newID, err)
		}

		result, err := dispatch.ExecuteAction(ctx, dispatch.ExecuteParams{
			DispatchConfig: dispatch.DispatchConfig{
				DB:                 database,
				NonInteractiveFunc: getWorkerFactory(),
				InteractiveFunc:    getInteractiveWorkerFactory(),
				RemoteFunc:         getRemoteWorkerFactory(),
				SessionLogChecker:  &dispatch.FileSessionLogChecker{},
				TmuxSession:        resumeSession,
			},
		}, action)

		var af *dispatch.ActionFailedError
		if errors.As(err, &af) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed (%v)\n", af.ActionID, af.Err)
			return nil
		}
		if err != nil {
			return err
		}

		switch result.Mode {
		case dispatch.ModeRemote:
			url := strings.TrimPrefix(result.Output, dispatch.RemoteSessionPrefix)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d resumed remotely from #%d (view: %s)\n", action.ID, parentID, url)
		case dispatch.ModeInteractive:
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d resumed interactively from #%d (%s)\n", action.ID, parentID, result.Output)
		default:
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d resumed from #%d (done)\n", action.ID, parentID)
		}
		return nil
	},
}

func init() {
	actionResumeCmd.Flags().StringVar(&resumeMessage, "message", db.DefaultResumeMessage,
		"Additional instruction passed as the new prompt for the resumed session")
	actionResumeCmd.Flags().StringVar(&resumeMode, "mode", "",
		"Execution mode: interactive | noninteractive | remote (default: parent action's mode)")
	actionResumeCmd.Flags().StringVar(&resumeSession, "session", "main",
		"Target tmux session name (interactive mode only)")
	actionCmd.AddCommand(actionResumeCmd)
}
