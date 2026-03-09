package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Task management",
}

var (
	taskProjectID int64
	taskTitle     string
	taskURL       string
	taskMeta      string
)

var taskCreateCmd = &cobra.Command{
	Use:   "create <TITLE>",
	Short: "Create a new task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskTitle = args[0]
		project, err := database.GetProjectByID(taskProjectID)
		if err != nil {
			return fmt.Errorf("project %d not found: %w", taskProjectID, err)
		}
		id, err := database.InsertTask(project.ID, taskTitle, taskURL, taskMeta)
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "task #%d created (project: %s)\n", id, project.Name)
		return nil
	},
}

var (
	taskListProjectID int64
	taskListStatus    string
)

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := database.ListTasks(taskListProjectID, taskListStatus)
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}
		if len(tasks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no tasks found")
			return nil
		}

		taskIDs := make([]int64, len(tasks))
		for i, t := range tasks {
			taskIDs[i] = t.ID
		}
		actionsByTask, err := database.ListActionsByTaskIDs(taskIDs)
		if err != nil {
			return fmt.Errorf("list actions: %w", err)
		}

		rows := make([]map[string]any, len(tasks))
		for i, t := range tasks {
			row := map[string]any{
				"id":         t.ID,
				"project_id": t.ProjectID,
				"title":      t.Title,
				"url":        t.URL,
				"metadata":   t.Metadata,
				"status":     t.Status,
				"created_at": t.CreatedAt,
			}
			if t.UpdatedAt.Valid {
				row["updated_at"] = t.UpdatedAt.String
			} else {
				row["updated_at"] = nil
			}

			actions := actionsByTask[t.ID]
			actionRows := make([]map[string]any, len(actions))
			for j, a := range actions {
				ar := map[string]any{
					"id":         a.ID,
					"prompt_id":  a.PromptID,
					"metadata":   a.Metadata,
					"status":     a.Status,
					"created_at": a.CreatedAt,
				}
				if a.TaskID.Valid {
					ar["task_id"] = a.TaskID.Int64
				} else {
					ar["task_id"] = nil
				}
				if a.Result.Valid {
					ar["result"] = a.Result.String
				} else {
					ar["result"] = nil
				}
				if a.SessionID.Valid {
					ar["session_id"] = a.SessionID.String
				} else {
					ar["session_id"] = nil
				}
				if a.StartedAt.Valid {
					ar["started_at"] = a.StartedAt.String
				} else {
					ar["started_at"] = nil
				}
				if a.CompletedAt.Valid {
					ar["completed_at"] = a.CompletedAt.String
				} else {
					ar["completed_at"] = nil
				}
				actionRows[j] = ar
			}
			row["actions"] = actionRows
			rows[i] = row
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	},
}

var (
	taskUpdateID        int64
	taskUpdateStatus    string
	taskUpdateProjectID int64
)

var taskUpdateCmd = &cobra.Command{
	Use:   "update <ID>",
	Short: "Update a task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		taskUpdateID, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid task ID: %w", err)
		}
		if taskUpdateStatus == "" && taskUpdateProjectID == 0 {
			return fmt.Errorf("at least one of --status or --project is required")
		}

		var updates []string

		if taskUpdateProjectID != 0 {
			project, err := database.GetProjectByID(taskUpdateProjectID)
			if err != nil {
				return fmt.Errorf("project %d not found: %w", taskUpdateProjectID, err)
			}
			if err := database.UpdateTaskProject(taskUpdateID, project.ID); err != nil {
				return fmt.Errorf("update task project: %w", err)
			}
			updates = append(updates, fmt.Sprintf("project: %s", project.Name))
		}

		if taskUpdateStatus != "" {
			if err := database.UpdateTask(taskUpdateID, taskUpdateStatus); err != nil {
				return fmt.Errorf("update task: %w", err)
			}
			updates = append(updates, fmt.Sprintf("status: %s", taskUpdateStatus))
		}

		fmt.Fprintf(cmd.OutOrStdout(), "task #%d updated (%s)\n", taskUpdateID, joinUpdates(updates))
		return nil
	},
}

func joinUpdates(updates []string) string {
	return strings.Join(updates, ", ")
}

func init() {
	taskCreateCmd.Flags().Int64Var(&taskProjectID, "project", 0, "Project ID (required)")
	taskCreateCmd.Flags().StringVar(&taskURL, "url", "", "Related URL")
	taskCreateCmd.Flags().StringVar(&taskMeta, "meta", "{}", "Metadata JSON")
	taskCreateCmd.MarkFlagRequired("project")

	taskUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "New status (open|review|done|blocked|archived)")
	taskUpdateCmd.Flags().Int64Var(&taskUpdateProjectID, "project", 0, "Project ID")

	taskListCmd.Flags().Int64Var(&taskListProjectID, "project", 0, "Filter by project ID")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "Filter by status")

	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskUpdateCmd)
}
