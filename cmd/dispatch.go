package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/prompt"
	"github.com/spf13/cobra"
)

var defaultWorkerFactory = func() dispatch.Worker {
	return &dispatch.NonInteractiveWorker{
		Runner: &dispatch.ExecRunner{},
	}
}

var activeWorkerFactory func() dispatch.Worker

func getWorkerFactory() func() dispatch.Worker {
	if activeWorkerFactory != nil {
		return activeWorkerFactory
	}
	return defaultWorkerFactory
}

func SetWorkerFactory(f func() dispatch.Worker) {
	activeWorkerFactory = f
}

var dispatchSession string

var defaultInteractiveWorkerFactory = func() dispatch.Worker {
	return &dispatch.InteractiveWorker{
		Runner:  &dispatch.ExecRunner{},
		Session: dispatchSession,
	}
}

var activeInteractiveWorkerFactory func() dispatch.Worker

func getInteractiveWorkerFactory() func() dispatch.Worker {
	if activeInteractiveWorkerFactory != nil {
		return activeInteractiveWorkerFactory
	}
	return defaultInteractiveWorkerFactory
}

func SetInteractiveWorkerFactory(f func() dispatch.Worker) {
	activeInteractiveWorkerFactory = f
}

var defaultRemoteWorkerFactory = func() dispatch.Worker {
	return &dispatch.RemoteWorker{
		Runner: &dispatch.ExecRunner{},
	}
}

var activeRemoteWorkerFactory func() dispatch.Worker

func getRemoteWorkerFactory() func() dispatch.Worker {
	if activeRemoteWorkerFactory != nil {
		return activeRemoteWorkerFactory
	}
	return defaultRemoteWorkerFactory
}

func SetRemoteWorkerFactory(f func() dispatch.Worker) {
	activeRemoteWorkerFactory = f
}

var dispatchCmd = &cobra.Command{
	Use:   "dispatch [action_id]",
	Short: "Dispatch next pending action",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var action *db.Action
		var err error

		if len(args) == 1 {
			id, parseErr := strconv.ParseInt(args[0], 10, 64)
			if parseErr != nil {
				return fmt.Errorf("invalid action ID %q: %w", args[0], parseErr)
			}
			action, err = database.ClaimPending(ctx, id)
			if err != nil {
				return err
			}
		} else {
			action, err = database.NextPending(ctx)
			if err != nil {
				return fmt.Errorf("next pending: %w", err)
			}
			if action == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "no pending actions")
				return nil
			}
		}

		promptData, err := buildPromptData(action)
		if err != nil {
			_ = database.MarkFailed(action.ID, fmt.Sprintf("build prompt data: %v", err))
			return fmt.Errorf("build prompt data: %w", err)
		}

		promptsDir := resolvePromptsDir()
		tmpl, err := prompt.Load(promptsDir, action.PromptID)
		if err != nil {
			_ = database.MarkFailed(action.ID, fmt.Sprintf("prompt load error: %v", err))
			return fmt.Errorf("load prompt: %w", err)
		}

		prompt, err := tmpl.Render(promptData)
		if err != nil {
			_ = database.MarkFailed(action.ID, fmt.Sprintf("render error: %v", err))
			return fmt.Errorf("render prompt: %w", err)
		}

		workDir := "."
		if promptData.Project.WorkDir != "" {
			workDir = promptData.Project.WorkDir
		}

		if tmpl.Config.IsRemote() {
			worker := getRemoteWorkerFactory()()
			result, err := worker.Execute(ctx, prompt, tmpl.Config, workDir, action.ID)
			if err != nil {
				_ = database.MarkFailed(action.ID, err.Error())
				fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed: %v\n", action.ID, err)
				return nil
			}
			if err := database.MergeActionMetadata(action.ID, map[string]any{
				"remote_session": result,
			}); err != nil {
				slog.Warn("failed to save remote session info", "action_id", action.ID, "error", err)
			}
			sessionURL := strings.TrimPrefix(result, dispatch.RemoteSessionPrefix)
			fmt.Fprintf(cmd.OutOrStdout(), "action #%d dispatched remotely\nView: %s\n", action.ID, sessionURL)
			return nil
		}

		if tmpl.Config.IsInteractive() {
			worker := getInteractiveWorkerFactory()()
			result, err := worker.Execute(ctx, prompt, tmpl.Config, workDir, action.ID)
			if err != nil {
				_ = database.MarkFailed(action.ID, err.Error())
				fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed: %v\n", action.ID, err)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "action #%d dispatched interactively: %s\n", action.ID, result)
			return nil
		}

		worker := getWorkerFactory()()
		result, err := worker.Execute(ctx, prompt, tmpl.Config, workDir, action.ID)
		if err != nil {
			_ = database.MarkFailed(action.ID, err.Error())
			fmt.Fprintf(cmd.OutOrStdout(), "action #%d failed: %v\n", action.ID, err)
			return nil
		}

		if err := database.MarkDone(action.ID, result); err != nil {
			return fmt.Errorf("mark done: %w", err)
		}

		if err := dispatch.TriggerOnDone(database, promptsDir, action, result); err != nil {
			slog.Warn("on_done trigger failed", "action_id", action.ID, "error", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "action #%d done\n", action.ID)
		return nil
	},
}

func buildPromptData(action *db.Action) (prompt.PromptData, error) {
	var data prompt.PromptData

	actionMeta := make(map[string]any)
	if action.Metadata != "" && action.Metadata != "{}" {
		if err := json.Unmarshal([]byte(action.Metadata), &actionMeta); err != nil {
			return data, fmt.Errorf("parse action metadata: %w", err)
		}
	}
	data.Action = prompt.ActionData{
		ID:         action.ID,
		PromptID: action.PromptID,
		Status:     action.Status,
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
		data.Task = prompt.TaskData{
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
		data.Project = prompt.ProjectData{
			ID:      project.ID,
			Name:    project.Name,
			WorkDir: project.WorkDir,
			Meta:    projectMeta,
		}
	}

	return data, nil
}

func init() {
	dispatchCmd.Flags().StringVar(&dispatchSession, "session", "main", "Target tmux session name")
}
