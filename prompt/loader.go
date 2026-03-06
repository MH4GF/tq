package template

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Description string `yaml:"description"`
	Interactive bool   `yaml:"interactive"`
	OnDone      string `yaml:"on_done"`
}

type Template struct {
	ID     string
	Config Config
	Body   string
}

type PromptData struct {
	Task    TaskData
	Project ProjectData
	Action  ActionData
}

type TaskData struct {
	ID     int64
	Title  string
	URL    string
	Status string
	Meta   map[string]any
}

type ProjectData struct {
	ID      int64
	Name    string
	WorkDir string
	Meta    map[string]any
}

type ActionData struct {
	ID         int64
	TemplateID string
	Status     string
	Source     string
	Meta       map[string]any
}

func Load(templatesDir, templateID string) (*Template, error) {
	path := filepath.Join(templatesDir, templateID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", templateID, err)
	}

	content := string(data)
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("template %q: missing frontmatter delimiters", templateID)
	}

	frontmatter := []byte(strings.TrimSpace(parts[1]))
	body := strings.TrimSpace(parts[2])

	var cfg Config
	if err := yaml.Unmarshal(frontmatter, &cfg); err != nil {
		return nil, fmt.Errorf("template %q: invalid YAML: %w", templateID, err)
	}

	return &Template{
		ID:     templateID,
		Config: cfg,
		Body:   body,
	}, nil
}

func (t *Template) Render(data PromptData) (string, error) {
	tmpl, err := template.New(t.ID).Parse(t.Body)
	if err != nil {
		return "", fmt.Errorf("template %q: parse error: %w", t.ID, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template %q: render error: %w", t.ID, err)
	}

	return buf.String(), nil
}
