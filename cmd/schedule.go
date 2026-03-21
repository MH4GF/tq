package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
)

var scheduleListLimit int
var scheduleListJQ string

var scheduleListFields = []string{"id", "task_id", "prompt_id", "title", "cron_expr", "metadata", "enabled", "last_run_at", "created_at"}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Create and manage scheduled actions",
}

var scheduleCreateCmd = &cobra.Command{
	Use:   "create <PROMPT_ID>",
	Short: "Create a new schedule",
	Long: `Create a scheduled action that runs on a cron schedule.

--task and --cron are required. PROMPT_ID is the prompt template to use.
--cron accepts standard 5-field cron expressions (minute hour dom month dow).`,
	Example: `  tq schedule create daily-review --task 1 --cron "0 9 * * *" --title "Morning review"
  tq schedule create sync-prs --task 2 --cron "*/30 * * * *"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		promptID := args[0]
		taskID, _ := cmd.Flags().GetInt64("task")
		title, _ := cmd.Flags().GetString("title")
		cronExpr, _ := cmd.Flags().GetString("cron")
		meta, _ := cmd.Flags().GetString("meta")

		if taskID == 0 {
			return fmt.Errorf("--task is required")
		}
		if cronExpr == "" {
			return fmt.Errorf("--cron is required")
		}
		if _, err := db.CronParser.Parse(cronExpr); err != nil {
			return fmt.Errorf("invalid cron expression %q: %w", cronExpr, err)
		}
		if title == "" {
			title = promptID
		}
		if err := validateMetaJSON(meta); err != nil {
			return err
		}

		id, err := database.InsertSchedule(taskID, promptID, title, cronExpr, meta)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "schedule #%d created\n", id)
		return nil
	},
}

var scheduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List schedules (JSON output)",
	RunE: func(cmd *cobra.Command, args []string) error {
		schedules, err := database.ListSchedules(scheduleListLimit)
		if err != nil {
			return err
		}

		rows := make([]map[string]any, len(schedules))
		for i, s := range schedules {
			row := map[string]any{
				"id":        s.ID,
				"task_id":   s.TaskID,
				"prompt_id": s.PromptID,
				"title":     s.Title,
				"cron_expr": s.CronExpr,
				"metadata":  s.Metadata,
				"enabled":   s.Enabled,
			}
			if s.LastRunAt.Valid {
				row["last_run_at"] = db.FormatLocal(s.LastRunAt.String)
			} else {
				row["last_run_at"] = nil
			}
			row["created_at"] = db.FormatLocal(s.CreatedAt)
			rows[i] = row
		}
		return WriteJSON(cmd.OutOrStdout(), rows, scheduleListJQ, scheduleListFields)
	},
}

var scheduleEnableCmd = &cobra.Command{
	Use:   "enable <ID>",
	Short: "Enable a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		if err := database.UpdateScheduleEnabled(id, true); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "schedule #%d enabled\n", id)
		return nil
	},
}

var scheduleDisableCmd = &cobra.Command{
	Use:   "disable <ID>",
	Short: "Disable a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		if err := database.UpdateScheduleEnabled(id, false); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "schedule #%d disabled\n", id)
		return nil
	},
}

var scheduleDeleteCmd = &cobra.Command{
	Use:   "delete <ID>",
	Short: "Delete a schedule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		if err := database.DeleteSchedule(id); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "schedule #%d deleted\n", id)
		return nil
	},
}

var scheduleUpdateCmd = &cobra.Command{
	Use:   "update <ID>",
	Short: "Update a schedule",
	Example: `  tq schedule update 1 --cron "0 10 * * *"
  tq schedule update 2 --title "Weekly sync" --task 3`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}

		var title, cronExpr, meta *string
		var taskID *int64

		if cmd.Flags().Changed("title") {
			v, _ := cmd.Flags().GetString("title")
			title = &v
		}
		if cmd.Flags().Changed("cron") {
			v, _ := cmd.Flags().GetString("cron")
			if _, err := db.CronParser.Parse(v); err != nil {
				return fmt.Errorf("invalid cron expression %q: %w", v, err)
			}
			cronExpr = &v
		}
		if cmd.Flags().Changed("meta") {
			v, _ := cmd.Flags().GetString("meta")
			if err := validateMetaJSON(v); err != nil {
				return err
			}
			meta = &v
		}
		if cmd.Flags().Changed("task") {
			v, _ := cmd.Flags().GetInt64("task")
			taskID = &v
		}

		if title == nil && cronExpr == nil && meta == nil && taskID == nil {
			return fmt.Errorf("at least one flag (--title, --cron, --meta, --task) is required")
		}

		if err := database.UpdateSchedule(id, title, cronExpr, meta, taskID); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "schedule #%d updated\n", id)
		return nil
	},
}

func parseID(s string) (int64, error) {
	var id int64
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid ID %q: %w", s, err)
	}
	return id, nil
}

func init() {
	scheduleCreateCmd.Flags().Int64("task", 0, "Task ID (required, see: tq task list)")
	scheduleCreateCmd.Flags().String("title", "", "Schedule title (defaults to prompt ID)")
	scheduleCreateCmd.Flags().String("cron", "", "Cron expression (required, e.g. \"0 9 * * *\")")
	scheduleCreateCmd.Flags().String("meta", "{}", `JSON metadata (e.g. {"key":"value"})`)

	scheduleListCmd.Flags().IntVar(&scheduleListLimit, "limit", 0, "Limit number of results (0 = no limit)")
	scheduleListCmd.Flags().StringVar(&scheduleListJQ, "jq", "", jqFlagUsage(scheduleListFields))

	scheduleCmd.AddCommand(scheduleCreateCmd)
	scheduleCmd.AddCommand(scheduleListCmd)
	scheduleCmd.AddCommand(scheduleEnableCmd)
	scheduleCmd.AddCommand(scheduleDisableCmd)
	scheduleCmd.AddCommand(scheduleDeleteCmd)

	scheduleUpdateCmd.Flags().String("title", "", "Schedule title")
	scheduleUpdateCmd.Flags().String("cron", "", "Cron expression")
	scheduleUpdateCmd.Flags().String("meta", "", "JSON metadata")
	scheduleUpdateCmd.Flags().Int64("task", 0, "Task ID")
	scheduleCmd.AddCommand(scheduleUpdateCmd)
}
