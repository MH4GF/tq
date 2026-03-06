package prompt

import (
	"os"
	"path/filepath"
	"testing"
)

func writePrompt(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_AllFields(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "full", `---
description: Full prompt
mode: interactive
on_done: review
---
Body content here.
`)

	p, err := Load(dir, "full")
	if err != nil {
		t.Fatal(err)
	}

	if p.ID != "full" {
		t.Errorf("ID = %q, want %q", p.ID, "full")
	}
	if p.Config.Description != "Full prompt" {
		t.Errorf("Description = %q, want %q", p.Config.Description, "Full prompt")
	}
	if !p.Config.IsInteractive() {
		t.Errorf("IsInteractive() = false, want true")
	}
	if p.Config.OnDone != "review" {
		t.Errorf("OnDone = %q, want %q", p.Config.OnDone, "review")
	}
	if p.Body != "Body content here." {
		t.Errorf("Body = %q, want %q", p.Body, "Body content here.")
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "minimal", `---
description: Minimal
---
Hello.
`)

	p, err := Load(dir, "minimal")
	if err != nil {
		t.Fatal(err)
	}

	if !p.Config.IsInteractive() {
		t.Errorf("IsInteractive() = false, want true (default)")
	}
	if p.Config.OnDone != "" {
		t.Errorf("OnDone = %q, want empty", p.Config.OnDone)
	}
}

func TestLoad_ModeNonInteractive(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "ni", `---
description: NonInteractive
mode: noninteractive
---
Body.
`)

	p, err := Load(dir, "ni")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Config.IsNonInteractive() {
		t.Errorf("IsNonInteractive() = false, want true")
	}
	if p.Config.IsInteractive() {
		t.Errorf("IsInteractive() = true, want false")
	}
}

func TestLoad_ModeRemote(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "rem", `---
description: Remote
mode: remote
---
Body.
`)

	p, err := Load(dir, "rem")
	if err != nil {
		t.Fatal(err)
	}
	if !p.Config.IsRemote() {
		t.Errorf("IsRemote() = false, want true")
	}
	if p.Config.IsInteractive() {
		t.Errorf("IsInteractive() = true, want false")
	}
}

func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent prompt")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "bad", `---
description: [invalid
  yaml: {{
---
Body.
`)

	_, err := Load(dir, "bad")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestRender_AllVariables(t *testing.T) {
	p := &Prompt{
		ID: "test",
		Body: `Task: {{.Task.ID}} {{.Task.Title}} {{.Task.URL}} {{.Task.Status}}
Project: {{.Project.ID}} {{.Project.Name}} {{.Project.WorkDir}}
Action: {{.Action.ID}} {{.Action.PromptID}} {{.Action.Status}} {{.Action.Source}}
TaskMeta: {{index .Task.Meta "key"}}
ProjectMeta: {{index .Project.Meta "key"}}
ActionMeta: {{index .Action.Meta "key"}}`,
	}

	data := PromptData{
		Task: TaskData{
			ID:     1,
			Title:  "Test Task",
			URL:    "https://example.com/1",
			Status: "open",
			Meta:   map[string]any{"key": "tval"},
		},
		Project: ProjectData{
			ID:      2,
			Name:    "MyProject",
			WorkDir: "/tmp/proj",
			Meta:    map[string]any{"key": "pval"},
		},
		Action: ActionData{
			ID:       3,
			PromptID: "implement",
			Status:   "pending",
			Source:    "github",
			Meta:     map[string]any{"key": "aval"},
		},
	}

	result, err := p.Render(data)
	if err != nil {
		t.Fatal(err)
	}

	expected := `Task: 1 Test Task https://example.com/1 open
Project: 2 MyProject /tmp/proj
Action: 3 implement pending github
TaskMeta: tval
ProjectMeta: pval
ActionMeta: aval`

	if result != expected {
		t.Errorf("Render result:\n%s\nwant:\n%s", result, expected)
	}
}

func TestRender_EmptyMeta(t *testing.T) {
	p := &Prompt{
		ID:   "simple",
		Body: "Hello {{.Task.Title}}",
	}

	data := PromptData{
		Task: TaskData{
			Title: "World",
			Meta:  map[string]any{},
		},
		Project: ProjectData{Meta: map[string]any{}},
		Action:  ActionData{Meta: map[string]any{}},
	}

	result, err := p.Render(data)
	if err != nil {
		t.Fatal(err)
	}

	if result != "Hello World" {
		t.Errorf("Render = %q, want %q", result, "Hello World")
	}
}
