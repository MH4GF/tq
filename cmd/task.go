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
	taskProject string
	taskTitle   string
	taskURL     string
	taskMeta    string
)

var taskCreateCmd = &cobra.Command{
	Use:   "create <TITLE>",
	Short: "Create a new task",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskTitle = args[0]
		project, err := database.GetProjectByName(taskProject)
		if err != nil {
			return fmt.Errorf("project %q not found: %w", taskProject, err)
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
	taskListProject string
	taskListStatus  string
)

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := database.ListTasks(taskListProject, taskListStatus)
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}
		if len(tasks) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no tasks found")
			return nil
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
			rows[i] = row
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	},
}

var (
	taskUpdateID      int64
	taskUpdateStatus  string
	taskUpdateProject string
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
		if taskUpdateStatus == "" && taskUpdateProject == "" {
			return fmt.Errorf("at least one of --status or --project is required")
		}

		var updates []string

		if taskUpdateProject != "" {
			project, err := database.GetProjectByName(taskUpdateProject)
			if err != nil {
				return fmt.Errorf("project %q not found: %w", taskUpdateProject, err)
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
	taskCreateCmd.Flags().StringVar(&taskProject, "project", "", "Project name (required)")
	taskCreateCmd.Flags().StringVar(&taskURL, "url", "", "Related URL")
	taskCreateCmd.Flags().StringVar(&taskMeta, "meta", "{}", "Metadata JSON")
	taskCreateCmd.MarkFlagRequired("project")

	taskUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "New status (open|review|done|blocked|archived)")
	taskUpdateCmd.Flags().StringVar(&taskUpdateProject, "project", "", "Project name")

	taskListCmd.Flags().StringVar(&taskListProject, "project", "", "Filter by project name")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "Filter by status")

	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskUpdateCmd)
}
