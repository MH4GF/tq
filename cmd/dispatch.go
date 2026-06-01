package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/dispatch"
)

var defaultBgWorkerFactory = func() dispatch.Worker {
	return &dispatch.BgWorker{Runner: &dispatch.ExecRunner{}}
}

var activeBgWorkerFactory func() dispatch.Worker

func getBgWorkerFactory() func() dispatch.Worker {
	if activeBgWorkerFactory != nil {
		return activeBgWorkerFactory
	}
	return defaultBgWorkerFactory
}

// SetBgWorkerFactory overrides the BgWorker factory used by `tq action dispatch`
// and the queue worker. Exposed for tests; production callers leave it unset.
func SetBgWorkerFactory(f func() dispatch.Worker) {
	activeBgWorkerFactory = f
}

var defaultRemoteWorkerFactory = func() dispatch.Worker {
	return &dispatch.RemoteWorker{
		Runner: &dispatch.ExecRunner{},
	}
}

var activeRemoteWorkerFactory func() dispatch.Worker

func getRemoteWorkerFactory() func() dispatch.Worker {
	if activeRemoteWorkerFactory != nil {
		return activeRemoteWorkerFactory
	}
	return defaultRemoteWorkerFactory
}

var actionDispatchCmd = &cobra.Command{
	Use:   "dispatch <action_id>",
	Short: "Dispatch an action immediately (skip queue)",
	Long: `Dispatch a pending action immediately by its ID.

This is a manual override: it bypasses the completion-dependency gate, so an
action whose blockers are not yet satisfied (including blocked-forever ones)
is dispatched anyway. The automatic queue worker still respects dependencies.`,
	Example: `  tq action dispatch 42`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, parseErr := strconv.ParseInt(args[0], 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid action ID %q: %w", args[0], parseErr)
		}
		cfg := dispatch.DispatchConfig{
			DB:         database,
			BgFunc:     getBgWorkerFactory(),
			RemoteFunc: getRemoteWorkerFactory(),
		}
		msg, err := dispatch.DispatchByID(ctx, cfg, id)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), msg)
		return nil
	},
}

func init() {
	actionCmd.AddCommand(actionDispatchCmd)
}
