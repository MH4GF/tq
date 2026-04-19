package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

func init() {
	actionCmd.AddCommand(failCmd)
}

var failCmd = &cobra.Command{
	Use:   "fail ACTION_ID [REASON]",
	Short: "Mark action as failed when the goal could not be achieved",
	Long: `Mark an action as failed. Use this when the action could not be completed
despite genuine effort (missing permissions, broken environment, external API
down, etc.). Can be called on any non-terminal action (pending, running, or
dispatched).

Distinction from other terminal commands:
  done    - Work completed successfully
  fail    - Work attempted but could not complete (retry may be possible)
  cancel  - Work intentionally aborted (not needed, superseded, etc.)

Failed actions can be reset to pending with ` + "`tq action reset`" + ` for retry.

REASON is free-form text explaining the failure. Structure it with these
sections (omit any that don't apply):

  outcome:    What could not be achieved - the concrete blocker
  decisions:  What was tried and why it did not work
  artifacts:  Partial PRs, files, log excerpts, error messages
  remaining:  What is needed to unblock - env fix, external response,
              retry conditions, etc.

Do NOT describe process ("I ran grep, then read the file...").
Session logs already capture that.`,
	Example: `  tq action fail 5

  tq action fail 5 'outcome: Could not merge PR #142 - CI stuck on flaky e2e test
  decisions: Retried CI twice; skipping the test locally is not acceptable per project rules
  artifacts: PR #142, CI run 98765
  remaining: Need e2e flake fix from infra team before retry'`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid action ID: %w", err)
		}

		action, err := database.GetAction(id)
		if err != nil {
			return fmt.Errorf("action #%d not found (see: tq action list): %w", id, err)
		}

		if action.Status == db.ActionStatusDone || action.Status == db.ActionStatusFailed || action.Status == db.ActionStatusCancelled {
			return fmt.Errorf("action #%d is already %q, cannot mark as failed (only pending, running, or dispatched actions can be failed)", id, action.Status)
		}

		reason := ""
		if len(args) > 1 {
			reason = args[1]
		}

		if err := database.MarkFailed(id, reason); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}

		if reason != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed (reason: %s)\n", id, strings.TrimSpace(reason))
		} else {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed\n", id)
		}
		return nil
	},
}
