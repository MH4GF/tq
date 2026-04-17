package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/dispatch"
)

var defaultWorkerFactory = func() dispatch.Worker {
	return &dispatch.NonInteractiveWorker{
		Runner: &dispatch.ExecRunner{},
	}
}

var activeWorkerFactory func() dispatch.Worker

func getWorkerFactory() func() dispatch.Worker {
	if activeWorkerFactory != nil {
		return activeWorkerFactory
	}
	return defaultWorkerFactory
}

func SetWorkerFactory(f func() dispatch.Worker) {
	activeWorkerFactory = f
}

var dispatchSession string

var defaultInteractiveWorkerFactory = func() dispatch.Worker {
	return &dispatch.InteractiveWorker{
		Runner:  &dispatch.ExecRunner{},
		Session: dispatchSession,
	}
}

var activeInteractiveWorkerFactory func() dispatch.Worker

func getInteractiveWorkerFactory() func() dispatch.Worker {
	if activeInteractiveWorkerFactory != nil {
		return activeInteractiveWorkerFactory
	}
	return defaultInteractiveWorkerFactory
}

func SetInteractiveWorkerFactory(f func() dispatch.Worker) {
	activeInteractiveWorkerFactory = f
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

func SetRemoteWorkerFactory(f func() dispatch.Worker) {
	activeRemoteWorkerFactory = f
}

var actionDispatchCmd = &cobra.Command{
	Use:   "dispatch <action_id>",
	Short: "Dispatch an action immediately (skip queue)",
	Long:  `Dispatch a pending action immediately by its ID.`,
	Example: `  tq action dispatch 42
  tq action dispatch 42 --session work`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		id, parseErr := strconv.ParseInt(args[0], 10, 64)
		if parseErr != nil {
			return fmt.Errorf("invalid action ID %q: %w", args[0], parseErr)
		}
		action, err := database.ClaimPending(ctx, id)
		if err != nil {
			return err
		}

		result, err := dispatch.ExecuteAction(ctx, dispatch.ExecuteParams{
			DispatchConfig: dispatch.DispatchConfig{
				DB:                 database,
				NonInteractiveFunc: getWorkerFactory(),
				InteractiveFunc:    getInteractiveWorkerFactory(),
				RemoteFunc:         getRemoteWorkerFactory(),
				SessionLogChecker:  &dispatch.FileSessionLogChecker{},
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
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d dispatched remotely (view: %s)\n", action.ID, url)
		case dispatch.ModeInteractive:
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d dispatched interactively (%s)\n", action.ID, result.Output)
		default:
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "action #%d done\n", action.ID)
		}
		return nil
	},
}

func init() {
	actionDispatchCmd.Flags().StringVar(&dispatchSession, "session", "main", "Target tmux session name")
	actionCmd.AddCommand(actionDispatchCmd)
}
