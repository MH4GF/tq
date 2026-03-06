package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
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
		logCh := make(chan tui.LogEntry, 100)

		prevLogger := slog.Default()
		handler := &tui.TUILogHandler{Ch: logCh, Level: slog.LevelInfo}
		slog.SetDefault(slog.New(handler))
		defer slog.SetDefault(prevLogger)

		notificationWriter := &tui.LogWriter{Ch: logCh}

		cfgDir, err := configDir()
		if err != nil {
			return err
		}

		ralphBg := func(ctx context.Context) error {
			cfg := dispatch.RalphConfig{
				UserConfigDir:  cfgDir,
				DB:             database,
				MaxInteractive: uiMaxInteractive,
				PollInterval:   uiPollInterval,
				TmuxChecker:    &dispatch.ExecTmuxChecker{Runner: &dispatch.ExecRunner{}},
				NonInteractiveFunc: func() dispatch.Worker {
					return &dispatch.NonInteractiveWorker{
						Runner: &dispatch.ExecRunner{},
					}
				},
				InteractiveFunc: func() dispatch.Worker {
					return &dispatch.InteractiveWorker{
						Runner: &dispatch.ExecRunner{},
					}
				},
				RemoteFunc: func() dispatch.Worker {
					return &dispatch.RemoteWorker{
						Runner: &dispatch.ExecRunner{},
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
					if err := runWatch(ctx, notificationWriter); err != nil {
						slog.Error("watch error", "error", err)
					}
				}
			}
		}

		m := tui.New(database, logCh, ralphBg, watchBg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tui: %w", err)
		}
		return nil
	},
}

func runWatch(ctx context.Context, notificationWriter io.Writer) error {
	src, err := ghsource.NewGitHubSource()
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}

	notifications, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	promptsDir := resolvePromptsDir()

	for _, n := range notifications {
		notifBytes, err := json.Marshal(n.Metadata)
		if err != nil {
			continue
		}

		if tryCompleteRemoteAction(n.Metadata, promptsDir) {
			_ = src.MarkProcessed(ctx, n)
			continue
		}

		if err := runClassifyGhNotification(notificationWriter, string(notifBytes)); err != nil {
			slog.Error("classify-gh-notification", "error", err)
			continue
		}
		_ = src.MarkProcessed(ctx, n)
	}
	return nil
}

var remoteActionBranchRe = regexp.MustCompile(`^tq-(\d+)-`)

func tryCompleteRemoteAction(metadata map[string]any, promptsDir string) bool {
	subjectType, _ := metadata["subject_type"].(string)
	if subjectType != "PullRequest" {
		return false
	}

	headBranch, _ := metadata["head_branch"].(string)
	if headBranch == "" {
		return false
	}

	matches := remoteActionBranchRe.FindStringSubmatch(headBranch)
	if matches == nil {
		return false
	}

	actionID, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return false
	}

	action, err := database.GetAction(actionID)
	if err != nil {
		slog.Warn("remote action lookup failed", "action_id", actionID, "error", err)
		return false
	}
	if action.Status != "running" {
		return false
	}

	prURL, _ := metadata["url"].(string)
	result := fmt.Sprintf("remote:pr=%s", prURL)

	if err := database.MarkDone(actionID, result); err != nil {
		slog.Error("mark remote action done", "action_id", actionID, "error", err)
		return false
	}

	if err := dispatch.TriggerOnDone(database, promptsDir, action, result); err != nil {
		slog.Warn("on_done trigger failed for remote action", "action_id", actionID, "error", err)
	}

	slog.Info("remote action completed via PR", "action_id", actionID, "branch", headBranch, "pr", prURL)
	return true
}

func init() {
	uiCmd.Flags().IntVar(&uiMaxInteractive, "max-interactive", 3, "Maximum concurrent interactive sessions")
	uiCmd.Flags().DurationVar(&uiPollInterval, "poll", 10*time.Second, "Ralph loop poll interval")
	uiCmd.Flags().DurationVar(&uiWatchInterval, "watch-interval", 5*time.Minute, "GitHub notification check interval")
}
