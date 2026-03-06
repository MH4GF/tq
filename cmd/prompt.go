package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/MH4GF/tq/prompt"
	"github.com/spf13/cobra"
)

var promptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Manage prompt templates",
}

var promptListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available prompt templates",
	RunE: func(cmd *cobra.Command, args []string) error {
		userDir := resolvePromptsDir()
		projectDir := "prompts"

		prompts, err := prompt.List(userDir, projectDir)
		if err != nil {
			return fmt.Errorf("list prompts: %w", err)
		}

		if len(prompts) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no prompts found")
			return nil
		}

		type row struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Mode        string `json:"mode"`
			OnDone      string `json:"on_done"`
			Scope       string `json:"scope"`
		}

		rows := make([]row, len(prompts))
		for i, p := range prompts {
			scope := "user"
			if _, err := prompt.Load(projectDir, p.ID); err == nil {
				scope = "project"
			}
			rows[i] = row{
				ID:          p.ID,
				Description: p.Config.Description,
				Mode:        p.Config.Mode,
				OnDone:      p.Config.OnDone,
				Scope:       scope,
			}
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	},
}

func init() {
	promptCmd.AddCommand(promptListCmd)
}
