package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/tui"
	"github.com/spf13/cobra"
)

var (
	uiMaxInteractive int
	uiPollInterval   time.Duration
	uiSession        string
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive TUI with queue worker",
	Long: `Launch the terminal UI with a queue worker that auto-dispatches pending actions via tmux.`,
	Example: `  tq ui
  tq ui --max-interactive 5 --poll 30s
  tq ui --session work`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logCh := make(chan tui.LogEntry, 100)

		prevLogger := slog.Default()
		handler := &tui.TUILogHandler{Ch: logCh, Level: slog.LevelInfo}
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(prevLogger)

		cfgDir, err := configDir()
		if err != nil {
			return err
		}

		workerBg := func(ctx context.Context) error {
			cfg := dispatch.WorkerConfig{
				DispatchConfig: dispatch.DispatchConfig{
					DB: database,
					NonInteractiveFunc: func() dispatch.Worker {
						return &dispatch.NonInteractiveWorker{
							Runner: &dispatch.ExecRunner{},
						}
					},
					InteractiveFunc: func() dispatch.Worker {
						return &dispatch.InteractiveWorker{
							Runner:  &dispatch.ExecRunner{},
							Session: uiSession,
						}
					},
					RemoteFunc: func() dispatch.Worker {
						return &dispatch.RemoteWorker{
							Runner: &dispatch.ExecRunner{},
						}
					},
					TmuxSession: uiSession,
				},
				UserConfigDir:  cfgDir,
				MaxInteractive: uiMaxInteractive,
				PollInterval:   uiPollInterval,
				TmuxChecker:    &dispatch.ExecTmuxChecker{Runner: &dispatch.ExecRunner{}},
			}
			return dispatch.RunWorker(ctx, cfg)
		}

		m := tui.New(database, logCh, workerBg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui: %w", err)
		}
		return nil
	},
}

func init() {
	uiCmd.Flags().IntVar(&uiMaxInteractive, "max-interactive", 3, "Maximum concurrent interactive sessions")
	uiCmd.Flags().DurationVar(&uiPollInterval, "poll", 10*time.Second, "Queue worker poll interval")
	uiCmd.Flags().StringVar(&uiSession, "session", "main", "Target tmux session name")
}
