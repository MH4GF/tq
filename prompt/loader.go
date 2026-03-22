package prompt

import (
	"bytes"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed internal/*.md
var internalPrompts embed.FS

type Config struct {
	Description    string `yaml:"description"`
	Mode           string `yaml:"mode"` // "interactive" (default) | "noninteractive" | "remote"
	PermissionMode string `yaml:"permission_mode"`
	Worktree       bool   `yaml:"-"` // set via action metadata, not frontmatter
	OnDone         string `yaml:"on_done"`
	OnCancel       string `yaml:"on_cancel"`
}

func (c Config) IsInteractive() bool    { return c.Mode == "interactive" }
func (c Config) IsNonInteractive() bool { return c.Mode == "noninteractive" }
func (c Config) IsRemote() bool         { return c.Mode == "remote" }

type Prompt struct {
	ID     string
	Config Config
	Body   string
}

type LoadResult struct {
	Prompt             *Prompt
	UnknownFields      []string
	DeprecatedPatterns []string
}

var knownFrontmatterKeys = func() map[string]bool {
	m := make(map[string]bool)
	t := reflect.TypeFor[Config]()
	for i := range t.NumField() {
		if tag := t.Field(i).Tag.Get("yaml"); tag != "" {
			m[tag] = true
		}
	}
	return m
}()

var (
	validPermissionMode      = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)
	deprecatedTaskURLPattern = regexp.MustCompile(`\{\{\s*\.Task\.URL\s*\}\}`)
)

type PromptData struct {
	Task    TaskData
	Project ProjectData
	Action  ActionData
}

type TaskData struct {
	ID      int64
	Title   string
	Status  string
	WorkDir string
	Meta    map[string]any
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

func Load(promptsDir, promptID string) (*LoadResult, error) {
	var data []byte
	var err error

	if name, ok := strings.CutPrefix(promptID, "internal:"); ok {
		data, err = internalPrompts.ReadFile("internal/" + name + ".md")
		if err != nil {
			return nil, fmt.Errorf("internal prompt %q not found: %w", promptID, err)
		}
	} else {
		path := filepath.Join(promptsDir, promptID+".md")
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("prompt %q not found: %w", promptID, err)
		}
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

	var rawMap map[string]any
	var unknownFields []string
	if err := yaml.Unmarshal(frontmatter, &rawMap); err == nil {
		for key := range rawMap {
			if !knownFrontmatterKeys[key] {
				unknownFields = append(unknownFields, key)
			}
		}
		if len(unknownFields) > 0 {
			sort.Strings(unknownFields)
			slog.Warn("unknown frontmatter fields", "prompt_id", promptID, "fields", unknownFields)
		}
	}

	if cfg.Mode == "" {
		cfg.Mode = "interactive"
	}

	if cfg.PermissionMode != "" && !validPermissionMode.MatchString(cfg.PermissionMode) {
		return nil, fmt.Errorf("prompt %q: invalid permission_mode %q (must be alphanumeric/hyphens)", promptID, cfg.PermissionMode)
	}

	var deprecatedPatterns []string
	if deprecatedTaskURLPattern.MatchString(body) {
		deprecatedPatterns = append(deprecatedPatterns, "{{.Task.URL}}")
	}

	return &LoadResult{
		Prompt: &Prompt{
			ID:     promptID,
			Config: cfg,
			Body:   body,
		},
		UnknownFields:      unknownFields,
		DeprecatedPatterns: deprecatedPatterns,
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
			result, err := Load(dir, id)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", filepath.Join(dir, e.Name()), err)
				continue
			}
			seen[id] = *result.Prompt
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
	tmpl, err := template.New(p.ID).
		Option("missingkey=error").
		Funcs(template.FuncMap{
			"index": strictIndex,
			"get":   softIndex,
		}).
		Parse(p.Body)
	if err != nil {
		return "", fmt.Errorf("prompt %q: parse error: %w", p.ID, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("prompt %q: render error: %w", p.ID, err)
	}

	return buf.String(), nil
}

func strictIndex(item any, keys ...any) (any, error) {
	v := reflect.ValueOf(item)
	for _, key := range keys {
		switch v.Kind() {
		case reflect.Map:
			result := v.MapIndex(reflect.ValueOf(key))
			if !result.IsValid() {
				return nil, fmt.Errorf("missing metadata key %q", key)
			}
			v = result
		default:
			idx := int(reflect.ValueOf(key).Int())
			if idx < 0 || idx >= v.Len() {
				return nil, fmt.Errorf("index %d out of range", idx)
			}
			v = v.Index(idx)
		}
	}
	return v.Interface(), nil
}

func softIndex(item any, keys ...any) (any, error) {
	v := reflect.ValueOf(item)
	for _, key := range keys {
		switch v.Kind() {
		case reflect.Map:
			result := v.MapIndex(reflect.ValueOf(key))
			if !result.IsValid() {
				return "", nil
			}
			v = result
		default:
			idx := int(reflect.ValueOf(key).Int())
			if idx < 0 || idx >= v.Len() {
				return nil, fmt.Errorf("index %d out of range", idx)
			}
			v = v.Index(idx)
		}
	}
	return v.Interface(), nil
}
