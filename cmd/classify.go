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
		recordClassifyFailure(notificationJSON, "", fmt.Sprintf("load classify template: %v", err))
		return fmt.Errorf("load classify template: %w", err)
	}

	existingTasks, err := buildExistingTasksList()
	if err != nil {
		recordClassifyFailure(notificationJSON, "", fmt.Sprintf("build tasks list: %v", err))
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
		recordClassifyFailure(notificationJSON, "", fmt.Sprintf("render template: %v", err))
		return fmt.Errorf("render template: %w", err)
	}

	ctx := context.Background()
	worker := getWorkerFactory()(tqDirResolved)
	result, err := worker.Execute(ctx, prompt, tmpl.Config, tqDirResolved, 0)
	if err != nil {
		recordClassifyFailure(notificationJSON, "", fmt.Sprintf("classify execution: %v", err))
		return fmt.Errorf("classify execution: %w", err)
	}

	if err := processClassifyResult(cmd, notificationJSON, result); err != nil {
		recordClassifyFailure(notificationJSON, result, err.Error())
		return err
	}
	return nil
}

func processClassifyResult(cmd *cobra.Command, notificationJSON, resultJSON string) error {
	var result ClassifyResult
	if err := json.Unmarshal([]byte(resultJSON), &result); err != nil {
		return fmt.Errorf("parse classify result: %w", err)
	}

	metadata := buildClassifyMetadata(notificationJSON, resultJSON)

	var taskID int64
	if result.Task.ID > 0 {
		_, err := database.GetTask(result.Task.ID)
		if err != nil {
			return fmt.Errorf("task #%d not found: %w", result.Task.ID, err)
		}
		taskID = result.Task.ID
		fmt.Fprintf(cmd.OutOrStdout(), "linked to existing task #%d\n", taskID)
	} else {
		if result.Task.ProjectName == "" {
			return fmt.Errorf("classify result has empty project_name for new task")
		}
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

		id, err := database.InsertAction(a.TemplateID, &taskID, metadata, status, a.Priority, "classify")
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d created (%s, status: %s)\n", id, a.TemplateID, status)
	}

	return nil
}

func buildClassifyMetadata(notificationJSON, llmOutput string) string {
	meta := map[string]any{}
	if notificationJSON != "" {
		if json.Valid([]byte(notificationJSON)) {
			meta["notification"] = json.RawMessage(notificationJSON)
		} else {
			meta["notification"] = notificationJSON
		}
	}
	if llmOutput != "" {
		if json.Valid([]byte(llmOutput)) {
			meta["classify_output"] = json.RawMessage(llmOutput)
		} else {
			meta["classify_output"] = llmOutput
		}
	}
	b, err := json.Marshal(meta)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func recordClassifyFailure(notificationJSON, llmOutput, errMsg string) {
	meta := buildClassifyMetadata(notificationJSON, llmOutput)
	id, err := database.InsertAction("classify", nil, meta, "failed", 0, "classify")
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
