package cmd

import (
	"fmt"
	"text/tabwriter"

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
	Use:   "create",
	Short: "Create a new task",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tProject\tStatus\tTitle\tURL")
		for _, t := range tasks {
			url := "-"
			if t.URL != "" {
				url = t.URL
			}
			fmt.Fprintf(w, "%d\t%d\t%s\t%s\t%s\n", t.ID, t.ProjectID, t.Status, t.Title, url)
		}
		return w.Flush()
	},
}

var (
	taskUpdateID     int64
	taskUpdateStatus string
)

var taskUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a task's status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := database.UpdateTask(taskUpdateID, taskUpdateStatus); err != nil {
			return fmt.Errorf("update task: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "task #%d updated (status: %s)\n", taskUpdateID, taskUpdateStatus)
		return nil
	},
}

func init() {
	taskCreateCmd.Flags().StringVar(&taskProject, "project", "", "Project name (required)")
	taskCreateCmd.Flags().StringVar(&taskTitle, "title", "", "Task title (required)")
	taskCreateCmd.Flags().StringVar(&taskURL, "url", "", "Related URL")
	taskCreateCmd.Flags().StringVar(&taskMeta, "meta", "{}", "Metadata JSON")
	taskCreateCmd.MarkFlagRequired("project")
	taskCreateCmd.MarkFlagRequired("title")

	taskUpdateCmd.Flags().Int64Var(&taskUpdateID, "id", 0, "Task ID (required)")
	taskUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "New status (open|review|done|blocked|archived)")
	taskUpdateCmd.MarkFlagRequired("id")
	taskUpdateCmd.MarkFlagRequired("status")

	taskListCmd.Flags().StringVar(&taskListProject, "project", "", "Filter by project name")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "Filter by status")

	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskUpdateCmd)
}
