package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Create, list, and update tasks",
}

var (
	taskProjectID int64
	taskTitle     string
	taskMeta      string
	taskWorkDir   string
)

var taskCreateCmd = &cobra.Command{
	Use:   "create <TITLE>",
	Short: "Create a new task",
	Long: `Create a new task under a project. --project is required.
--work-dir overrides the project's default working directory for this task.
URL and other extra data can be stored in --meta (e.g. --meta '{"url":"https://..."}').`,
	Example: `  tq task create "Fix login bug" --project 1
  tq task create "Review PR #99" --project 2 --meta '{"url":"https://github.com/org/repo/pull/99"}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		taskTitle = args[0]
		if err := validateMetaJSON(taskMeta); err != nil {
			return err
		}
		project, err := database.GetProjectByID(taskProjectID)
		if err != nil {
			return fmt.Errorf("project %d not found (see: tq project list): %w", taskProjectID, err)
		}
		workDir := taskWorkDir
		if workDir == "" {
			workDir = project.WorkDir
		}
		id, err := database.InsertTask(project.ID, taskTitle, taskMeta, workDir)
		if err != nil {
			return fmt.Errorf("insert task: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "task #%d created (project: %s)\n", id, project.Name)
		return nil
	},
}

var (
	taskListProjectID int64
	taskListStatus    string
	taskListLimit     int
	taskListJQ        string
)

var taskListFields = []string{"id", "project_id", "title", "metadata", "status", "work_dir", "created_at", "updated_at", "actions"}

var taskListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tasks (JSON output, includes nested actions)",
	Example: `  tq task list
  tq task list --project 1
  tq task list --status open
  tq task list --project 2 --status done`,
	RunE: func(cmd *cobra.Command, args []string) error {
		tasks, err := database.ListTasks(taskListProjectID, taskListStatus, taskListLimit)
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
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
			rows[i] = taskToMap(t, actionsByTask[t.ID])
		}
		return WriteJSON(cmd.OutOrStdout(), rows, taskListJQ, taskListFields)
	},
}

var (
	taskUpdateID        int64
	taskUpdateStatus    string
	taskUpdateProjectID int64
	taskUpdateWorkDir   string
	taskUpdateMeta      string
)

var taskUpdateCmd = &cobra.Command{
	Use:   "update <ID>",
	Short: "Update a task",
	Long: `Update a task's status, project, working directory, or metadata.
At least one of --status, --project, --work-dir, or --meta is required.`,
	Example: `  tq task update 1 --status done
  tq task update 3 --status archived
  tq task update 5 --project 2 --work-dir ~/src/other
  tq task update 7 --meta '{"url":"https://github.com/org/repo/pull/99"}'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		taskUpdateID, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid task ID: %w", err)
		}
		if taskUpdateStatus == "" && taskUpdateProjectID == 0 && taskUpdateWorkDir == "" && taskUpdateMeta == "" {
			return fmt.Errorf("at least one of --status, --project, --work-dir, or --meta is required")
		}

		var metaMap map[string]any
		if taskUpdateMeta != "" {
			if err := json.Unmarshal([]byte(taskUpdateMeta), &metaMap); err != nil {
				return fmt.Errorf("invalid JSON for --meta (must be a JSON object): %s", taskUpdateMeta)
			}
		}

		var updates []string

		if taskUpdateProjectID != 0 {
			project, err := database.GetProjectByID(taskUpdateProjectID)
			if err != nil {
				return fmt.Errorf("project %d not found (see: tq project list): %w", taskUpdateProjectID, err)
			}
			if err := database.UpdateTaskProject(taskUpdateID, project.ID); err != nil {
				return fmt.Errorf("update task project: %w", err)
			}
			updates = append(updates, fmt.Sprintf("project: %s", project.Name))
		}

		if taskUpdateWorkDir != "" {
			if err := database.UpdateTaskWorkDir(taskUpdateID, taskUpdateWorkDir); err != nil {
				return fmt.Errorf("update task work_dir: %w", err)
			}
			updates = append(updates, fmt.Sprintf("work_dir: %s", taskUpdateWorkDir))
		}

		if metaMap != nil {
			if err := database.MergeTaskMetadata(taskUpdateID, metaMap); err != nil {
				return fmt.Errorf("update task metadata: %w", err)
			}
			updates = append(updates, "metadata: updated")
		}

		if taskUpdateStatus != "" {
			if err := database.UpdateTask(taskUpdateID, taskUpdateStatus, ""); err != nil {
				return fmt.Errorf("update task: %w", err)
			}
			updates = append(updates, fmt.Sprintf("status: %s", taskUpdateStatus))
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "task #%d updated (%s)\n", taskUpdateID, joinUpdates(updates))
		return nil
	},
}

func taskToMap(t db.Task, actions []db.Action) map[string]any {
	row := map[string]any{
		"id":         t.ID,
		"project_id": t.ProjectID,
		"title":      t.Title,
		"metadata":   t.Metadata,
		"status":     t.Status,
		"work_dir":   t.WorkDir,
		"created_at": db.FormatLocal(t.CreatedAt),
	}
	if t.UpdatedAt.Valid {
		row["updated_at"] = db.FormatLocal(t.UpdatedAt.String)
	} else {
		row["updated_at"] = nil
	}
	actionRows := make([]map[string]any, len(actions))
	for i, a := range actions {
		actionRows[i] = actionToMap(a)
	}
	row["actions"] = actionRows
	return row
}

var taskGetJQ string

var taskGetCmd = &cobra.Command{
	Use:   "get <ID>",
	Short: "Get a task by ID (JSON output, includes nested actions)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		task, err := database.GetTask(id)
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}
		actions, err := database.ListActions("", &id, 0)
		if err != nil {
			return fmt.Errorf("list actions: %w", err)
		}
		return WriteJSON(cmd.OutOrStdout(), taskToMap(*task, actions), taskGetJQ, taskListFields)
	},
}

func joinUpdates(updates []string) string {
	return strings.Join(updates, ", ")
}

func init() {
	taskCreateCmd.Flags().Int64Var(&taskProjectID, "project", 0, "Project ID (required, see: tq project list)")
	taskCreateCmd.Flags().StringVar(&taskMeta, "meta", "{}", `JSON metadata (e.g. {"url":"https://...","key":"value"})`)
	taskCreateCmd.Flags().StringVar(&taskWorkDir, "work-dir", "", "Working directory (defaults to project work_dir)")
	if err := taskCreateCmd.MarkFlagRequired("project"); err != nil {
		panic(err)
	}
	taskCreateCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.Root().HelpFunc()(cmd, args)
		writeProjectHint(cmd.OutOrStdout())
	})

	taskUpdateCmd.Flags().StringVar(&taskUpdateStatus, "status", "", "New status (open, done, archived)")
	taskUpdateCmd.Flags().Int64Var(&taskUpdateProjectID, "project", 0, "Project ID")
	taskUpdateCmd.Flags().StringVar(&taskUpdateWorkDir, "work-dir", "", "Working directory")
	taskUpdateCmd.Flags().StringVar(&taskUpdateMeta, "meta", "", `JSON metadata to merge (e.g. {"url":"https://..."})`)

	taskListCmd.Flags().Int64Var(&taskListProjectID, "project", 0, "Filter by project ID (see: tq project list)")
	taskListCmd.Flags().StringVar(&taskListStatus, "status", "", "Filter by status (open, done, archived)")
	taskListCmd.Flags().IntVar(&taskListLimit, "limit", 0, "Limit number of results (0 = no limit)")
	taskListCmd.Flags().StringVar(&taskListJQ, "jq", "", jqFlagUsage(taskListFields))

	taskCmd.AddCommand(taskListCmd)
	taskCmd.AddCommand(taskCreateCmd)
	taskCmd.AddCommand(taskUpdateCmd)
	taskGetCmd.Flags().StringVar(&taskGetJQ, "jq", "", jqFlagUsage(taskListFields))
	taskCmd.AddCommand(taskGetCmd)
}
