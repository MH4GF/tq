package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/MH4GF/tq/dispatch"
)

func validateMetaJSON(meta string) error {
	var obj map[string]any
	if err := json.Unmarshal([]byte(meta), &obj); err != nil {
		return fmt.Errorf("invalid JSON for --meta (must be a JSON object): %s", meta)
	}
	for _, key := range []string{"permission_mode", "worktree"} {
		if _, exists := obj[key]; exists {
			return fmt.Errorf("metadata key %q is no longer supported; use claude_args instead (e.g. \"claude_args\":[\"--permission-mode\",\"plan\"])", key)
		}
	}
	if rawArgs, exists := obj[dispatch.MetaKeyClaudeArgs]; exists {
		arr, ok := rawArgs.([]any)
		if !ok {
			return fmt.Errorf("claude_args must be a JSON array of strings, got %T", rawArgs)
		}
		strs := make([]string, 0, len(arr))
		for i, v := range arr {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("claude_args[%d] must be a string, got %T", i, v)
			}
			strs = append(strs, s)
		}
		if err := dispatch.ValidateClaudeArgs(strs); err != nil {
			return err
		}
	}
	return nil
}
