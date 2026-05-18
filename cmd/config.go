package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
)

// knownConfigKeys restricts settings to recognized keys so a typo is rejected
// instead of silently stored and never read. Each entry validates its value.
var knownConfigKeys = map[string]func(string) error{
	db.SettingDefaultMode: func(v string) error {
		if !dispatch.IsValidMode(v) {
			return fmt.Errorf(
				"invalid value %q for %s: must be one of %s",
				v, db.SettingDefaultMode, dispatch.ValidModesList(),
			)
		}
		return nil
	},
}

func sortedConfigKeys() []string {
	keys := make([]string, 0, len(knownConfigKeys))
	for k := range knownConfigKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func knownConfigKeyList() string {
	return strings.Join(sortedConfigKeys(), ", ")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Get and set global tq settings (stored in the DB)",
	Long: `Global key-value settings stored in the DB, so they travel with
libsql/Turso endpoints rather than a local file.

Keys:
  default_mode  Default execution mode stamped into new actions when
                'tq action create --meta' does not specify one. One of
                interactive, noninteractive, remote, experimental_bg.
                An explicit --meta '{"mode":...}' always overrides it.`,
}

var configSetCmd = &cobra.Command{
	Use:     "set <key> <value>",
	Short:   "Set a setting",
	Example: `  tq config set default_mode experimental_bg`,
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, value := args[0], args[1]
		validate, ok := knownConfigKeys[key]
		if !ok {
			return fmt.Errorf("unknown config key %q (known: %s)", key, knownConfigKeyList())
		}
		if err := validate(value); err != nil {
			return err
		}
		if err := database.SetSetting(key, value); err != nil {
			return fmt.Errorf("set setting: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "config %s = %s\n", key, value)
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:     "get <key>",
	Short:   "Get a setting (empty when unset)",
	Example: `  tq config get default_mode`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if _, ok := knownConfigKeys[key]; !ok {
			return fmt.Errorf("unknown config key %q (known: %s)", key, knownConfigKeyList())
		}
		value, err := database.GetSetting(key)
		if err != nil {
			return fmt.Errorf("get setting: %w", err)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	},
}

var configListJQ string

var configListFields = []string{"key", "value"}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all known settings with their current values (JSON output)",
	RunE: func(cmd *cobra.Command, args []string) error {
		stored, err := database.ListSettings()
		if err != nil {
			return fmt.Errorf("list settings: %w", err)
		}

		keys := sortedConfigKeys()
		rows := make([]map[string]any, len(keys))
		for i, k := range keys {
			rows[i] = map[string]any{"key": k, "value": stored[k]}
		}
		return WriteJSON(cmd.OutOrStdout(), rows, configListJQ, configListFields)
	},
}

func init() {
	configListCmd.Flags().StringVar(&configListJQ, "jq", "", jqFlagUsage(configListFields))
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
}
