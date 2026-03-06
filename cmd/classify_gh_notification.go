package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/prompt"
	"github.com/spf13/cobra"
)

var classifyGhNotificationCmd = &cobra.Command{
	Use:   "classify-gh-notification [NOTIFICATION_JSON]",
	Short: "Classify a GitHub notification into task + actions",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var notificationJSON string
		if len(args) > 0 {
			notificationJSON = args[0]
		} else {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			notificationJSON = string(data)
		}

		return runClassifyGhNotification(cmd.OutOrStdout(), notificationJSON)
	},
}

func runClassifyGhNotification(w io.Writer, notificationJSON string) error {
	promptsDir := resolvePromptsDir()
	tmpl, err := prompt.Load(promptsDir, "classify-gh-notification")
	if err != nil {
		recordClassifyGhNotificationFailure(notificationJSON, fmt.Sprintf("load classify-gh-notification prompt: %v", err))
		return fmt.Errorf("load classify-gh-notification prompt: %w", err)
	}

	existingTasks, err := buildExistingTasksList()
	if err != nil {
		recordClassifyGhNotificationFailure(notificationJSON, fmt.Sprintf("build tasks list: %v", err))
		return fmt.Errorf("build tasks list: %w", err)
	}

	promptData := prompt.PromptData{
		Action: prompt.ActionData{
			Meta: map[string]any{
				"notification":   notificationJSON,
				"existing_tasks": existingTasks,
			},
		},
	}
	prompt, err := tmpl.Render(promptData)
	if err != nil {
		recordClassifyGhNotificationFailure(notificationJSON, fmt.Sprintf("render prompt: %v", err))
		return fmt.Errorf("render prompt: %w", err)
	}

	ctx := context.Background()
	var worker dispatch.Worker
	if tmpl.Config.IsInteractive() {
		worker = getInteractiveWorkerFactory()()
	} else {
		worker = getWorkerFactory()()
	}
	result, err := worker.Execute(ctx, prompt, tmpl.Config, ".", 0)
	if err != nil {
		recordClassifyGhNotificationFailure(notificationJSON, fmt.Sprintf("classify-gh-notification execution: %v", err))
		return fmt.Errorf("classify-gh-notification execution: %w", err)
	}

	fmt.Fprintln(w, result)
	return nil
}

func recordClassifyGhNotificationFailure(notificationJSON, errMsg string) {
	meta := "{}"
	if notificationJSON != "" {
		m := map[string]any{}
		if json.Valid([]byte(notificationJSON)) {
			m["notification"] = json.RawMessage(notificationJSON)
		} else {
			m["notification"] = notificationJSON
		}
		if b, err := json.Marshal(m); err == nil {
			meta = string(b)
		}
	}
	id, err := database.InsertAction("classify-gh-notification", nil, meta, "failed", "classify-gh-notification")
	if err != nil {
		return
	}
	_ = database.MarkFailed(id, errMsg)
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
