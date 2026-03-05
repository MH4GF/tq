package template

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemplate(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad_AllFields(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "full", `---
description: Full template
interactive: true
on_done: review
---
Body content here.
`)

	tmpl, err := Load(dir, "full")
	if err != nil {
		t.Fatal(err)
	}

	if tmpl.ID != "full" {
		t.Errorf("ID = %q, want %q", tmpl.ID, "full")
	}
	if tmpl.Config.Description != "Full template" {
		t.Errorf("Description = %q, want %q", tmpl.Config.Description, "Full template")
	}
	if tmpl.Config.Interactive != true {
		t.Errorf("Interactive = %v, want true", tmpl.Config.Interactive)
	}
	if tmpl.Config.OnDone != "review" {
		t.Errorf("OnDone = %q, want %q", tmpl.Config.OnDone, "review")
	}
	if tmpl.Body != "Body content here." {
		t.Errorf("Body = %q, want %q", tmpl.Body, "Body content here.")
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "minimal", `---
description: Minimal
---
Hello.
`)

	tmpl, err := Load(dir, "minimal")
	if err != nil {
		t.Fatal(err)
	}

	if tmpl.Config.Interactive != false {
		t.Errorf("Interactive = %v, want false", tmpl.Config.Interactive)
	}
	if tmpl.Config.OnDone != "" {
		t.Errorf("OnDone = %q, want empty", tmpl.Config.OnDone)
	}
}

func TestLoad_NotFound(t *testing.T) {
	dir := t.TempDir()

	_, err := Load(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent template")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeTemplate(t, dir, "bad", `---
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
	tmpl := &Template{
		ID: "test",
		Body: `Task: {{.Task.ID}} {{.Task.Title}} {{.Task.URL}} {{.Task.Status}}
Project: {{.Project.ID}} {{.Project.Name}} {{.Project.WorkDir}}
Action: {{.Action.ID}} {{.Action.TemplateID}} {{.Action.Status}} {{.Action.Source}}
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
			ID:         3,
			TemplateID: "implement",
			Status:     "pending",
			Source:      "github",
			Meta:       map[string]any{"key": "aval"},
		},
	}

	result, err := tmpl.Render(data)
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
	tmpl := &Template{
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

	result, err := tmpl.Render(data)
	if err != nil {
		t.Fatal(err)
	}

	if result != "Hello World" {
		t.Errorf("Render = %q, want %q", result, "Hello World")
	}
}
