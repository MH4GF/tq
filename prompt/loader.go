package prompt

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Description string `yaml:"description"`
	Mode        string `yaml:"mode"` // "interactive" (default) | "noninteractive" | "remote"
	OnDone      string `yaml:"on_done"`
}

func (c Config) IsInteractive() bool    { return c.Mode == "interactive" }
func (c Config) IsNonInteractive() bool { return c.Mode == "noninteractive" }
func (c Config) IsRemote() bool         { return c.Mode == "remote" }

type Prompt struct {
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
	ID       int64
	PromptID string
	Status   string
	Meta     map[string]any
}

func Load(promptsDir, promptID string) (*Prompt, error) {
	path := filepath.Join(promptsDir, promptID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("prompt %q not found: %w", promptID, err)
	}

	content := string(data)
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("prompt %q: missing frontmatter delimiters", promptID)
	}

	frontmatter := []byte(strings.TrimSpace(parts[1]))
	body := strings.TrimSpace(parts[2])

	var cfg Config
	if err := yaml.Unmarshal(frontmatter, &cfg); err != nil {
		return nil, fmt.Errorf("prompt %q: invalid YAML: %w", promptID, err)
	}

	if cfg.Mode == "" {
		cfg.Mode = "interactive"
	}

	return &Prompt{
		ID:     promptID,
		Config: cfg,
		Body:   body,
	}, nil
}

// List scans the given directories for .md prompt files, loads their frontmatter,
// and returns them sorted by ID. Later directories override earlier ones for duplicate IDs.
func List(dirs ...string) ([]Prompt, error) {
	seen := make(map[string]Prompt)
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read dir %q: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".md")
			p, err := Load(dir, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", filepath.Join(dir, e.Name()), err)
				continue
			}
			seen[id] = *p
		}
	}

	prompts := make([]Prompt, 0, len(seen))
	for _, p := range seen {
		prompts = append(prompts, p)
	}
	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].ID < prompts[j].ID
	})
	return prompts, nil
}

func (p *Prompt) Render(data PromptData) (string, error) {
	tmpl, err := template.New(p.ID).Parse(p.Body)
	if err != nil {
		return "", fmt.Errorf("prompt %q: parse error: %w", p.ID, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompt %q: render error: %w", p.ID, err)
	}

	return buf.String(), nil
}
