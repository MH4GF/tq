package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/template"
	"github.com/spf13/cobra"
)

var classifyCmd = &cobra.Command{
	Use:   "classify [NOTIFICATION_JSON]",
	Short: "Classify a notification into task + actions",
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

		return runClassify(cmd.OutOrStdout(), notificationJSON)
	},
}

func runClassify(w io.Writer, notificationJSON string) error {
	templatesDir := resolveTemplatesDir("")
	tmpl, err := template.Load(templatesDir, "classify")
	if err != nil {
		recordClassifyFailure(notificationJSON, fmt.Sprintf("load classify template: %v", err))
		return fmt.Errorf("load classify template: %w", err)
	}

	existingTasks, err := buildExistingTasksList()
	if err != nil {
		recordClassifyFailure(notificationJSON, fmt.Sprintf("build tasks list: %v", err))
		return fmt.Errorf("build tasks list: %w", err)
	}

	promptData := template.PromptData{
		Action: template.ActionData{
			Meta: map[string]any{
				"notification":   notificationJSON,
				"existing_tasks": existingTasks,
			},
		},
	}
	prompt, err := tmpl.Render(promptData)
	if err != nil {
		recordClassifyFailure(notificationJSON, fmt.Sprintf("render template: %v", err))
		return fmt.Errorf("render template: %w", err)
	}

	ctx := context.Background()
	var worker dispatch.Worker
	if tmpl.Config.Interactive {
		worker = getInteractiveWorkerFactory()()
	} else {
		worker = getWorkerFactory()()
	}
	result, err := worker.Execute(ctx, prompt, tmpl.Config, ".", 0)
	if err != nil {
		recordClassifyFailure(notificationJSON, fmt.Sprintf("classify execution: %v", err))
		return fmt.Errorf("classify execution: %w", err)
	}

	fmt.Fprintln(w, result)
	return nil
}

func recordClassifyFailure(notificationJSON, errMsg string) {
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
	id, err := database.InsertAction("classify", nil, meta, "failed", "classify")
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
