package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/template"
	"github.com/spf13/cobra"
)

var defaultWorkerFactory = func(tqDir string) dispatch.Worker {
	return &dispatch.NonInteractiveWorker{
		Runner: &dispatch.ExecRunner{},
		TQDir:  tqDir,
	}
}

var activeWorkerFactory func(string) dispatch.Worker

func getWorkerFactory() func(string) dispatch.Worker {
	if activeWorkerFactory != nil {
		return activeWorkerFactory
	}
	return defaultWorkerFactory
}

func SetWorkerFactory(f func(string) dispatch.Worker) {
	activeWorkerFactory = f
}

var defaultInteractiveWorkerFactory = func(tqDir string) dispatch.Worker {
	return &dispatch.InteractiveWorker{
		Runner: &dispatch.ExecRunner{},
		TQDir:  tqDir,
	}
}

var activeInteractiveWorkerFactory func(string) dispatch.Worker

func getInteractiveWorkerFactory() func(string) dispatch.Worker {
	if activeInteractiveWorkerFactory != nil {
		return activeInteractiveWorkerFactory
	}
	return defaultInteractiveWorkerFactory
}

func SetInteractiveWorkerFactory(f func(string) dispatch.Worker) {
	activeInteractiveWorkerFactory = f
}

var dispatchCmd = &cobra.Command{
	Use:   "dispatch",
	Short: "Dispatch next pending action",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		action, err := database.NextPending(ctx)
		if err != nil {
			return fmt.Errorf("next pending: %w", err)
		}
		if action == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "no pending actions")
			return nil
		}

		templatesDir := filepath.Join(tqDirResolved, "templates")
		tmpl, err := template.Load(templatesDir, action.TemplateID)
		if err != nil {
			_ = database.MarkFailed(action.ID, fmt.Sprintf("template load error: %v", err))
			return fmt.Errorf("load template: %w", err)
		}

		promptData, err := buildPromptData(action)
		if err != nil {
			_ = database.MarkFailed(action.ID, fmt.Sprintf("build prompt data: %v", err))
			return fmt.Errorf("build prompt data: %w", err)
		}

		prompt, err := tmpl.Render(promptData)
		if err != nil {
			_ = database.MarkFailed(action.ID, fmt.Sprintf("render error: %v", err))
			return fmt.Errorf("render template: %w", err)
		}

		workDir := tqDirResolved
		if promptData.Project.WorkDir != "" {
			workDir = promptData.Project.WorkDir
		}

		if tmpl.Config.Interactive {
			worker := getInteractiveWorkerFactory()(tqDirResolved)
			result, err := worker.Execute(ctx, prompt, tmpl.Config, workDir, action.ID)
			if err != nil {
				_ = database.MarkFailed(action.ID, err.Error())
				fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed: %v\n", action.ID, err)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "action #%d dispatched interactively: %s\n", action.ID, result)
			return nil
		}

		worker := getWorkerFactory()(tqDirResolved)
		result, err := worker.Execute(ctx, prompt, tmpl.Config, workDir, action.ID)
		if err != nil {
			_ = database.MarkFailed(action.ID, err.Error())
			fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed: %v\n", action.ID, err)
			return nil
		}

		if err := database.MarkDone(action.ID, result); err != nil {
			return fmt.Errorf("mark done: %w", err)
		}

		if err := dispatch.TriggerOnDone(database, templatesDir, action, result); err != nil {
			slog.Warn("on_done trigger failed", "action_id", action.ID, "error", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "action #%d done\n", action.ID)
		return nil
	},
}

func buildPromptData(action *db.Action) (template.PromptData, error) {
	var data template.PromptData

	actionMeta := make(map[string]any)
	if action.Metadata != "" && action.Metadata != "{}" {
		if err := json.Unmarshal([]byte(action.Metadata), &actionMeta); err != nil {
			return data, fmt.Errorf("parse action metadata: %w", err)
		}
	}
	data.Action = template.ActionData{
		ID:         action.ID,
		TemplateID: action.TemplateID,
		Status:     action.Status,
		Priority:   action.Priority,
		Source:     action.Source,
		Meta:       actionMeta,
	}

	if action.TaskID.Valid {
		task, err := database.GetTask(action.TaskID.Int64)
		if err != nil {
			return data, fmt.Errorf("get task: %w", err)
		}
		taskMeta := make(map[string]any)
		if task.Metadata != "" && task.Metadata != "{}" {
			if err := json.Unmarshal([]byte(task.Metadata), &taskMeta); err != nil {
				return data, fmt.Errorf("parse task metadata: %w", err)
			}
		}
		data.Task = template.TaskData{
			ID:     task.ID,
			Title:  task.Title,
			URL:    task.URL,
			Status: task.Status,
			Meta:   taskMeta,
		}

		project, err := database.GetProjectByID(task.ProjectID)
		if err != nil {
			return data, fmt.Errorf("get project: %w", err)
		}
		projectMeta := make(map[string]any)
		if project.Metadata != "" && project.Metadata != "{}" {
			if err := json.Unmarshal([]byte(project.Metadata), &projectMeta); err != nil {
				return data, fmt.Errorf("parse project metadata: %w", err)
			}
		}
		data.Project = template.ProjectData{
			ID:      project.ID,
			Name:    project.Name,
			WorkDir: project.WorkDir,
			Meta:    projectMeta,
		}
	}

	return data, nil
}
