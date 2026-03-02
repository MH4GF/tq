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
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start Ralph Loop (continuous dispatch)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		fmt.Fprintf(cmd.OutOrStdout(), "starting ralph loop (max_interactive=%d, poll=%s)\n", runMaxInteractive, runPollInterval)

		cfg := dispatch.RalphConfig{
			TQDir:          tqDirResolved,
			DB:             database,
			MaxInteractive: runMaxInteractive,
			PollInterval:   runPollInterval,
			NonInteractiveFunc: func(tqDir string) dispatch.Worker {
				return &dispatch.NonInteractiveWorker{
					Runner: &dispatch.ExecRunner{},
					TQDir:  tqDir,
				}
			},
			InteractiveFunc: func(tqDir string) dispatch.Worker {
				return &dispatch.InteractiveWorker{
					Runner: &dispatch.ExecRunner{},
					TQDir:  tqDir,
				}
			},
		}

		err := dispatch.RalphLoop(ctx, cfg)
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
}
