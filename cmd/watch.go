package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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
	Short: "Fetch GitHub notifications and create classify actions",
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

		projectID, err := database.EnsureNotificationsProject()
		if err != nil {
			return fmt.Errorf("ensure notifications project: %w", err)
		}
		taskID, err := database.GetOrCreateTriageTask(projectID)
		if err != nil {
			return fmt.Errorf("get or create triage task: %w", err)
		}
		existingTasks, err := buildExistingTasksList()
		if err != nil {
			return fmt.Errorf("build tasks list: %w", err)
		}

		var processed, failed int
		for _, n := range notifications {
			notifJSON, err := json.Marshal(n.Metadata)
			if err != nil {
				slog.Error("marshal notification metadata", "error", err)
				failed++
				continue
			}

			if _, err := createClassifyAction(string(notifJSON), taskID, existingTasks); err != nil {
				slog.Error("create classify action", "error", err, "title", n.Metadata["title"])
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
			return fmt.Errorf("all %d notifications failed to create classify action", failed)
		}
		return nil
	},
}

func buildExistingTasksList() (string, error) {
	tasks, err := database.ListTasksByStatus("open")
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "No existing open tasks.", nil
	}
	var lines []string
	for _, t := range tasks {
		project, _ := database.GetProjectByID(t.ProjectID)
		projectName := "unknown"
		if project != nil {
			projectName = project.Name
		}
		line := fmt.Sprintf("- #%d [%s] %s", t.ID, projectName, t.Title)
		if t.URL != "" {
			line += " " + t.URL
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n"), nil
}

// CreateClassifyAction creates a classify action with automatic project/task setup.
func CreateClassifyAction(notificationJSON string) (int64, error) {
	projectID, err := database.EnsureNotificationsProject()
	if err != nil {
		return 0, fmt.Errorf("ensure notifications project: %w", err)
	}

	taskID, err := database.GetOrCreateTriageTask(projectID)
	if err != nil {
		return 0, fmt.Errorf("get or create triage task: %w", err)
	}

	existingTasks, err := buildExistingTasksList()
	if err != nil {
		return 0, fmt.Errorf("build tasks list: %w", err)
	}

	return createClassifyAction(notificationJSON, taskID, existingTasks)
}

func createClassifyAction(notificationJSON string, taskID int64, existingTasks string) (int64, error) {
	meta := map[string]any{
		"existing_tasks": existingTasks,
	}
	if json.Valid([]byte(notificationJSON)) {
		meta["notification"] = json.RawMessage(notificationJSON)
	} else {
		meta["notification"] = notificationJSON
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return 0, fmt.Errorf("marshal metadata: %w", err)
	}

	id, err := database.InsertAction("classify-gh-notification", "classify-gh-notification", taskID, string(metaBytes), "pending")
	if err != nil {
		return 0, fmt.Errorf("insert action: %w", err)
	}

	return id, nil
}
