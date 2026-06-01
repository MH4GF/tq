package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/tui"
)

var (
	uiMaxInteractive    int
	uiMaxNonInteractive int
	uiPollInterval      time.Duration
)

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Launch interactive TUI with queue worker",
	Long: `Launch the terminal UI with a queue worker that auto-dispatches pending actions.

All local actions are dispatched as background sessions via 'claude --bg' and
appear in 'claude agents'. Remote-mode actions still go through 'claude --remote'.`,
	Example: `  tq ui
  tq ui --max-interactive 5 --poll 30s`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logCh := make(chan tui.LogEntry, 100)

		prevLogger := slog.Default()
		handler := &tui.TUILogHandler{Ch: logCh, Level: slog.LevelInfo}
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(prevLogger)

		effectiveMaxInteractive := uiMaxInteractive
		if effectiveMaxInteractive <= 0 {
			effectiveMaxInteractive = dispatch.DefaultMaxInteractive
		}
		effectiveMaxNonInteractive := uiMaxNonInteractive
		if effectiveMaxNonInteractive <= 0 {
			effectiveMaxNonInteractive = dispatch.DefaultMaxNonInteractive
		}

		dispatchCfg := dispatch.DispatchConfig{
			DB:         database,
			BgFunc:     getBgWorkerFactory(),
			RemoteFunc: getRemoteWorkerFactory(),
		}

		workerBg := func(ctx context.Context) error {
			cfg := dispatch.WorkerConfig{
				DispatchConfig:    dispatchCfg,
				MaxInteractive:    effectiveMaxInteractive,
				MaxNonInteractive: effectiveMaxNonInteractive,
				PollInterval:      uiPollInterval,
				BgStateReader:     dispatch.FileBgStateReader{},
			}
			return dispatch.RunWorker(ctx, cfg)
		}

		dispatchFn := func(ctx context.Context, id int64) (string, error) {
			return dispatch.DispatchByID(ctx, dispatchCfg, id)
		}

		m := tui.New(database, logCh, effectiveMaxInteractive, dispatchFn, workerBg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui: %w", err)
		}
		return nil
	},
}

func init() {
	uiCmd.Flags().IntVar(&uiMaxInteractive, "max-interactive", dispatch.DefaultMaxInteractive, "Maximum concurrent user-facing sessions (interactive slot pool), cognitive-load cap")
	uiCmd.Flags().IntVar(&uiMaxNonInteractive, "max-noninteractive", dispatch.DefaultMaxNonInteractive, "Maximum concurrent noninteractive sessions (separate slot pool)")
	uiCmd.Flags().DurationVar(&uiPollInterval, "poll", dispatch.DefaultPollInterval, "Queue worker poll interval")
}
