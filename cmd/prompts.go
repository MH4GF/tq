package cmd

import (
	"os"
	"path/filepath"
)

func resolvePromptsDir() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}

	promptsDir := filepath.Join(dir, "prompts")
	templatesDir := filepath.Join(dir, "templates")

	if _, err := os.Stat(promptsDir); os.IsNotExist(err) {
		if _, err := os.Stat(templatesDir); err == nil {
			_ = os.Rename(templatesDir, promptsDir)
		}
	}

	return promptsDir
}
