package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Create, list, and manage projects",
}

var projectCreateMeta string

var projectCreateCmd = &cobra.Command{
	Use:   "create <NAME> <WORK_DIR>",
	Short: "Create a new project",
	Long: `Create a new project. NAME is a short identifier, WORK_DIR is the absolute path where tasks run.`,
	Example: `  tq project create myapp ~/src/myapp
  tq project create infra ~/src/infra --meta '{"team":"platform"}'`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, workDir := args[0], args[1]
		if err := validateMetaJSON(projectCreateMeta); err != nil {
			return err
		}
		id, err := database.InsertProject(name, workDir, projectCreateMeta)
		if err != nil {
			return fmt.Errorf("insert project: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "project #%d created (%s)\n", id, name)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects (JSON output)",
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := database.ListProjects()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		if len(projects) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
			return nil
		}

		rows := make([]map[string]any, len(projects))
		for i, p := range projects {
			rows[i] = map[string]any{
				"id":               p.ID,
				"name":             p.Name,
				"work_dir":         p.WorkDir,
				"metadata":         p.Metadata,
				"dispatch_enabled": p.DispatchEnabled,
				"created_at":       p.CreatedAt,
			}
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	},
}

var projectDeleteCmd = &cobra.Command{
	Use:     "delete <ID>",
	Short:   "Delete a project",
	Example: `  tq project delete 2`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}
		if err := database.DeleteProject(id); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "project #%d deleted\n", id)
		return nil
	},
}

var (
	projectUpdateDispatchEnabled string
	projectUpdateWorkDir         string
)

var projectUpdateCmd = &cobra.Command{
	Use:     "update <ID>",
	Short:   "Update a project",
	Example: `  tq project update 1 --dispatch-enabled true
  tq project update 1 --work-dir ~/src/myapp-v2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := parseID(args[0])
		if err != nil {
			return err
		}

		if projectUpdateDispatchEnabled == "" && projectUpdateWorkDir == "" {
			return fmt.Errorf("at least one flag (--dispatch-enabled, --work-dir) is required")
		}

		if _, err := database.GetProjectByID(id); err != nil {
			return fmt.Errorf("get project: %w", err)
		}

		if projectUpdateDispatchEnabled != "" {
			enabled, err := strconv.ParseBool(projectUpdateDispatchEnabled)
			if err != nil {
				return fmt.Errorf("invalid --dispatch-enabled value: %w", err)
			}
			if err := database.SetDispatchEnabled(id, enabled); err != nil {
				return fmt.Errorf("set dispatch_enabled: %w", err)
			}
			state := "enabled"
			if !enabled {
				state = "disabled"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "project #%d updated (dispatch: %s)\n", id, state)
		}

		if projectUpdateWorkDir != "" {
			if err := database.SetWorkDir(id, projectUpdateWorkDir); err != nil {
				return fmt.Errorf("set work_dir: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "project #%d updated (work_dir: %s)\n", id, projectUpdateWorkDir)
		}

		return nil
	},
}

func init() {
	projectCreateCmd.Flags().StringVar(&projectCreateMeta, "meta", "{}", `JSON metadata (e.g. {"team":"platform"})`)
	projectUpdateCmd.Flags().StringVar(&projectUpdateDispatchEnabled, "dispatch-enabled", "", "Enable or disable dispatch (true/false)")
	projectUpdateCmd.Flags().StringVar(&projectUpdateWorkDir, "work-dir", "", "Set the working directory")

	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectUpdateCmd)
}
