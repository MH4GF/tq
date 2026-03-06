package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"github.com/MH4GF/tq/source"
	ghsource "github.com/MH4GF/tq/source/github"
	"github.com/spf13/cobra"
)

var watchSourceFactory func() (source.Source, error)

func getWatchSource() (source.Source, error) {
	if watchSourceFactory != nil {
		return watchSourceFactory()
	}
	return ghsource.NewGitHubSource()
}

func SetWatchSourceFactory(f func() (source.Source, error)) {
	watchSourceFactory = f
}

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Fetch GitHub notifications and classify GitHub notifications",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		src, err := getWatchSource()
		if err != nil {
			return fmt.Errorf("create source: %w", err)
		}

		notifications, err := src.Fetch(ctx)
		if err != nil {
			return fmt.Errorf("fetch notifications: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "fetched %d notifications from %s\n", len(notifications), src.Name())

		var processed, failed int
		for _, n := range notifications {
			notifJSON, err := json.Marshal(n.Metadata)
			if err != nil {
				slog.Error("marshal notification metadata", "error", err)
				failed++
				continue
			}

			if err := classifyGhNotification(cmd.OutOrStdout(), string(notifJSON)); err != nil {
				slog.Error("classify-gh-notification", "error", err, "title", n.Metadata["title"])
				failed++
				continue
			}

			if err := src.MarkProcessed(ctx, n); err != nil {
				slog.Error("mark processed", "error", err)
			}
			processed++
		}

		fmt.Fprintf(cmd.OutOrStdout(), "processed %d, failed %d\n", processed, failed)

		if len(notifications) > 0 && failed == len(notifications) {
			return fmt.Errorf("all %d notifications failed to classify-gh-notification", failed)
		}
		return nil
	},
}

func classifyGhNotification(w io.Writer, notificationJSON string) error {
	return runClassifyGhNotification(w, notificationJSON)
}
