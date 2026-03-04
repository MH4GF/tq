package cmd

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Project management",
}

var (
	projectCreateName    string
	projectCreateWorkDir string
	projectCreateMeta    string
)

var projectCreateCmd = &cobra.Command{
	Use:   "create <NAME> <WORK_DIR>",
	Short: "Create a new project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectCreateName = args[0]
		projectCreateWorkDir = args[1]
		id, err := database.InsertProject(projectCreateName, projectCreateWorkDir, projectCreateMeta)
		if err != nil {
			return fmt.Errorf("insert project: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "project #%d created (%s)\n", id, projectCreateName)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := database.ListProjects()
		if err != nil {
			return fmt.Errorf("list projects: %w", err)
		}
		if len(projects) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no projects found")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tName\tWorkDir\tMetadata")
		for _, p := range projects {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", p.ID, p.Name, p.WorkDir, p.Metadata)
		}
		return w.Flush()
	},
}

var projectDeleteID int64

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <ID>",
	Short: "Delete a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var err error
		projectDeleteID, err = strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid project ID: %w", err)
		}
		if err := database.DeleteProject(projectDeleteID); err != nil {
			return fmt.Errorf("delete project: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "project #%d deleted\n", projectDeleteID)
		return nil
	},
}

func init() {
	projectCreateCmd.Flags().StringVar(&projectCreateMeta, "metadata", "{}", "Metadata JSON")

	projectCmd.AddCommand(projectCreateCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectDeleteCmd)
}
