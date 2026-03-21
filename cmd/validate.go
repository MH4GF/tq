package cmd

import (
	"encoding/json"
	"fmt"
)

func validateMetaJSON(meta string) error {
	var obj map[string]any
	if err := json.Unmarshal([]byte(meta), &obj); err != nil {
		return fmt.Errorf("invalid JSON for --meta (must be a JSON object): %s", meta)
	}
	return nil
}

func instructionFromMeta(metadata string) string {
	var m map[string]any
	if err := json.Unmarshal([]byte(metadata), &m); err != nil {
		return ""
	}
	if v, ok := m["instruction"].(string); ok {
		return v
	}
	return ""
}
