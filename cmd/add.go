package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/MH4GF/tq/template"
	"github.com/spf13/cobra"
)

var (
	addTemplate string
	addTask     int64
	addMeta     string
	addPriority int
	addSource   string
	addStatus   string
	addForce    bool
)

var addCmd = &cobra.Command{
	Use:   "create <TEMPLATE>",
	Short: "Create an action",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		addTemplate = args[0]
		status := addStatus
		if status == "" {
			templatesDir := filepath.Join(tqDirResolved, "templates")
			tmpl, err := template.Load(templatesDir, addTemplate)
			if err != nil {
				return fmt.Errorf("load template: %w", err)
			}
			status = "pending"
			if !tmpl.Config.Auto {
				status = "waiting_human"
			}
		}

		var taskIDPtr *int64
		if addTask > 0 {
			taskIDPtr = &addTask

			if !addForce {
				dup, err := database.HasActiveAction(addTask, addTemplate)
				if err != nil {
					return fmt.Errorf("check duplicates: %w", err)
				}
				if dup {
					return fmt.Errorf("blocked: active action already exists for task %d template %s (use --force to override)", addTask, addTemplate)
				}
			}
		}

		id, err := database.InsertAction(addTemplate, taskIDPtr, addMeta, status, addPriority, addSource)
		if err != nil {
			return fmt.Errorf("insert action: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "action #%d created (status: %s)\n", id, status)
		return nil
	},
}

func init() {
	addCmd.Flags().Int64Var(&addTask, "task", 0, "Task ID")
	addCmd.Flags().StringVar(&addMeta, "meta", "{}", "Metadata JSON")
	addCmd.Flags().IntVar(&addPriority, "priority", 0, "Priority")
	addCmd.Flags().StringVar(&addSource, "source", "human", "Source")
	addCmd.Flags().StringVar(&addStatus, "status", "", "Override status (pending|done|running|failed|waiting_human)")
	addCmd.Flags().BoolVar(&addForce, "force", false, "Skip duplicate check")
	actionCmd.AddCommand(addCmd)
}
