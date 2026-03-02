package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/MH4GF/tq/template"
	"github.com/spf13/cobra"
)

// ClassifyResult represents the structured output from classify.
type ClassifyResult struct {
	Task struct {
		ID          int64  `json:"id"`
		ProjectName string `json:"project_name"`
		Title       string `json:"title"`
		URL         string `json:"url"`
	} `json:"task"`
	Actions []struct {
		TemplateID string `json:"template_id"`
		Priority   int    `json:"priority"`
	} `json:"actions"`
}

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

		return runClassify(cmd, notificationJSON)
	},
}

func runClassify(cmd *cobra.Command, notificationJSON string) error {
	templatesDir := filepath.Join(tqDirResolved, "templates")
	tmpl, err := template.Load(templatesDir, "classify")
	if err != nil {
		return fmt.Errorf("load classify template: %w", err)
	}

	existingTasks, err := buildExistingTasksList()
	if err != nil {
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
		return fmt.Errorf("render template: %w", err)
	}

	ctx := context.Background()
	worker := getWorkerFactory()(tqDirResolved)
	result, err := worker.Execute(ctx, prompt, tmpl.Config, tqDirResolved, 0)
	if err != nil {
		return fmt.Errorf("classify execution: %w", err)
	}

	return processClassifyResult(cmd, result)
}

func processClassifyResult(cmd *cobra.Command, resultJSON string) error {
	var result ClassifyResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("parse classify result: %w", err)
	}

	var taskID int64
	if result.Task.ID > 0 {
		_, err := database.GetTask(result.Task.ID)
		if err != nil {
			return fmt.Errorf("task #%d not found: %w", result.Task.ID, err)
		}
		taskID = result.Task.ID
		fmt.Fprintf(cmd.OutOrStdout(), "linked to existing task #%d\n", taskID)
	} else {
		project, err := database.GetProjectByName(result.Task.ProjectName)
		if err != nil {
			return fmt.Errorf("project %q not found: %w", result.Task.ProjectName, err)
		}
		var errInsert error
		taskID, errInsert = database.InsertTask(project.ID, result.Task.Title, result.Task.URL, "{}")
		if errInsert != nil {
			return fmt.Errorf("insert task: %w", errInsert)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "task #%d created (project: %s)\n", taskID, project.Name)
	}

	for _, a := range result.Actions {
		dup, err := database.HasPendingOrRunning(taskID, a.TemplateID)
		if err != nil {
			return fmt.Errorf("check duplicates: %w", err)
		}
		if dup {
			fmt.Fprintf(cmd.OutOrStdout(), "skipped duplicate action: %s for task #%d\n", a.TemplateID, taskID)
			continue
		}

		templatesDir := filepath.Join(tqDirResolved, "templates")
		status := "pending"
		tmpl, err := template.Load(templatesDir, a.TemplateID)
		if err == nil && !tmpl.Config.Auto {
			status = "waiting_human"
		}

		id, err := database.InsertAction(a.TemplateID, &taskID, "{}", status, a.Priority, "classify")
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d created (%s, status: %s)\n", id, a.TemplateID, status)
	}

	return nil
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
