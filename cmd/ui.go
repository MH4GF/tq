package cmd

import (
	"context"
	"fmt"
	"encoding/json"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/dispatch"
	ghsource "github.com/MH4GF/tq/source/github"
	"github.com/MH4GF/tq/tui"
	"github.com/spf13/cobra"
)

var (
	uiMaxInteractive int
	uiPollInterval   time.Duration
	uiWatchInterval  time.Duration
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive TUI with ralph loop and watch",
	RunE: func(cmd *cobra.Command, args []string) error {
		ralphBg := func(ctx context.Context) error {
			cfg := dispatch.RalphConfig{
				TQDir:          tqDirResolved,
				DB:             database,
				MaxInteractive: uiMaxInteractive,
				PollInterval:   uiPollInterval,
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
			return dispatch.RalphLoop(ctx, cfg)
		}

		watchBg := func(ctx context.Context) error {
			ticker := time.NewTicker(uiWatchInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
					if err := runWatch(ctx); err != nil {
						slog.Error("watch error", "error", err)
					}
				}
			}
		}

		m := tui.New(database, tqDirResolved, ralphBg, watchBg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui: %w", err)
		}
		return nil
	},
}

func runWatch(ctx context.Context) error {
	src, err := ghsource.NewGitHubSource()
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}

	notifications, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	for _, n := range notifications {
		notifBytes, err := json.Marshal(n.Metadata)
		if err != nil {
			continue
		}
		if err := runClassify(classifyCmd, string(notifBytes)); err != nil {
			slog.Error("classify", "error", err)
			continue
		}
		_ = src.MarkProcessed(ctx, n)
	}
	return nil
}

func init() {
	uiCmd.Flags().IntVar(&uiMaxInteractive, "max-interactive", 3, "Maximum concurrent interactive sessions")
	uiCmd.Flags().DurationVar(&uiPollInterval, "poll", 10*time.Second, "Ralph loop poll interval")
	uiCmd.Flags().DurationVar(&uiWatchInterval, "watch-interval", 5*time.Minute, "GitHub notification check interval")
}
