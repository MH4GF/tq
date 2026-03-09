package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/MH4GF/tq/dispatch"
	"github.com/spf13/cobra"
)

var (
	runMaxInteractive int
	runPollInterval   time.Duration
	runSession        string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start Ralph Loop (continuous dispatch)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		fmt.Fprintf(cmd.OutOrStdout(), "starting ralph loop (max_interactive=%d, poll=%s)\n", runMaxInteractive, runPollInterval)

		cfgDir, err := configDir()
		if err != nil {
			return err
		}

		cfg := dispatch.RalphConfig{
			UserConfigDir:  cfgDir,
			DB:             database,
			MaxInteractive: runMaxInteractive,
			PollInterval:   runPollInterval,
			TmuxSession:    runSession,
			NonInteractiveFunc: func() dispatch.Worker {
				return &dispatch.NonInteractiveWorker{
					Runner: &dispatch.ExecRunner{},
				}
			},
			InteractiveFunc: func() dispatch.Worker {
				return &dispatch.InteractiveWorker{
					Runner:  &dispatch.ExecRunner{},
					Session: runSession,
				}
			},
			RemoteFunc: func() dispatch.Worker {
				return &dispatch.RemoteWorker{
					Runner: &dispatch.ExecRunner{},
				}
			},
		}

		err = dispatch.RalphLoop(ctx, cfg)
		if err != nil && err != context.Canceled {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "ralph loop stopped")
		return nil
	},
}

func init() {
	runCmd.Flags().IntVar(&runMaxInteractive, "max-interactive", 3, "Maximum concurrent interactive sessions")
	runCmd.Flags().DurationVar(&runPollInterval, "poll", 10*time.Second, "Poll interval when idle")
	runCmd.Flags().StringVar(&runSession, "session", "main", "Target tmux session name")
}
