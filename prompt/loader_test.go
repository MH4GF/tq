package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePrompt(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(content), 0o644); err != nil {
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

	lr, err := Load(dir, "full")
	if err != nil {
		t.Fatal(err)
	}
	p := lr.Prompt

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
	if len(lr.UnknownFields) != 0 {
		t.Errorf("UnknownFields = %v, want empty", lr.UnknownFields)
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "minimal", `---
description: Minimal
---
Hello.
`)

	lr, err := Load(dir, "minimal")
	if err != nil {
		t.Fatal(err)
	}
	p := lr.Prompt

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

	lr, err := Load(dir, "ni")
	if err != nil {
		t.Fatal(err)
	}
	if !lr.Prompt.Config.IsNonInteractive() {
		t.Errorf("IsNonInteractive() = false, want true")
	}
	if lr.Prompt.Config.IsInteractive() {
		t.Errorf("IsInteractive() = true, want false")
	}
}

func TestLoad_PermissionMode(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "plan", `---
description: Plan prompt
mode: interactive
permission_mode: plan
on_done: implement
---
Body.
`)

	lr, err := Load(dir, "plan")
	if err != nil {
		t.Fatal(err)
	}
	if lr.Prompt.Config.PermissionMode != "plan" {
		t.Errorf("PermissionMode = %q, want %q", lr.Prompt.Config.PermissionMode, "plan")
	}
	if len(lr.UnknownFields) != 0 {
		t.Errorf("UnknownFields = %v, want empty", lr.UnknownFields)
	}
}

func TestLoad_InvalidPermissionMode(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "bad-perm", `---
description: Bad permission mode
permission_mode: "plan; rm -rf /"
---
Body.
`)

	_, err := Load(dir, "bad-perm")
	if err == nil {
		t.Fatal("expected error for invalid permission_mode")
	}
	if !strings.Contains(err.Error(), "invalid permission_mode") {
		t.Errorf("error = %q, want to contain 'invalid permission_mode'", err.Error())
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

	lr, err := Load(dir, "rem")
	if err != nil {
		t.Fatal(err)
	}
	if !lr.Prompt.Config.IsRemote() {
		t.Errorf("IsRemote() = false, want true")
	}
	if lr.Prompt.Config.IsInteractive() {
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

func TestList_MultipleDirs(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	writePrompt(t, dir1, "alpha", `---
description: Alpha
mode: interactive
---
Body.
`)
	writePrompt(t, dir2, "beta", `---
description: Beta
mode: noninteractive
---
Body.
`)

	prompts, err := List(dir1, dir2)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 prompts, got %d", len(prompts))
	}
	if prompts[0].ID != "alpha" {
		t.Errorf("first prompt ID = %q, want %q", prompts[0].ID, "alpha")
	}
	if prompts[1].ID != "beta" {
		t.Errorf("second prompt ID = %q, want %q", prompts[1].ID, "beta")
	}
}

func TestList_DuplicateIDOverride(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	writePrompt(t, dir1, "same", `---
description: From dir1
---
Body.
`)
	writePrompt(t, dir2, "same", `---
description: From dir2
---
Body.
`)

	prompts, err := List(dir1, dir2)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if prompts[0].Config.Description != "From dir2" {
		t.Errorf("description = %q, want %q (later dir should override)", prompts[0].Config.Description, "From dir2")
	}
}

func TestList_NonexistentDir(t *testing.T) {
	prompts, err := List("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected no error for nonexistent dir, got %v", err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected 0 prompts, got %d", len(prompts))
	}
}

func TestList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	prompts, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 0 {
		t.Fatalf("expected 0 prompts, got %d", len(prompts))
	}
}

func TestList_InvalidFileSkipped(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "good", `---
description: Good
---
Body.
`)
	writePrompt(t, dir, "bad", `no frontmatter here`)

	prompts, err := List(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt (bad skipped), got %d", len(prompts))
	}
	if prompts[0].ID != "good" {
		t.Errorf("prompt ID = %q, want %q", prompts[0].ID, "good")
	}
}

func TestRender_AllVariables(t *testing.T) {
	p := &Prompt{
		ID: "test",
		Body: `Task: {{.Task.ID}} {{.Task.Title}} {{.Task.Status}}
Project: {{.Project.ID}} {{.Project.Name}} {{.Project.WorkDir}}
Action: {{.Action.ID}} {{.Action.PromptID}} {{.Action.Status}}
TaskMeta: {{index .Task.Meta "key"}}
ProjectMeta: {{index .Project.Meta "key"}}
ActionMeta: {{index .Action.Meta "key"}}`,
	}

	data := PromptData{
		Task: TaskData{
			ID:     1,
			Title:  "Test Task",
			Status: "open",
			Meta:   map[string]any{"key": "tval", "url": "https://example.com/1"},
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
			Meta:     map[string]any{"key": "aval"},
		},
	}

	result, err := p.Render(data)
	if err != nil {
		t.Fatal(err)
	}

	expected := `Task: 1 Test Task open
Project: 2 MyProject /tmp/proj
Action: 3 implement pending
TaskMeta: tval
ProjectMeta: pval
ActionMeta: aval`

	if result != expected {
		t.Errorf("Render result:\n%s\nwant:\n%s", result, expected)
	}
}

func TestLoad_DeprecatedPatterns(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "old-style", `---
description: Uses deprecated URL
mode: interactive
---
URL: {{.Task.URL}}
`)

	lr, err := Load(dir, "old-style")
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.DeprecatedPatterns) != 1 {
		t.Fatalf("DeprecatedPatterns = %v, want 1 entry", lr.DeprecatedPatterns)
	}
	if lr.DeprecatedPatterns[0] != "{{.Task.URL}}" {
		t.Errorf("DeprecatedPatterns[0] = %q, want %q", lr.DeprecatedPatterns[0], "{{.Task.URL}}")
	}
}

func TestLoad_NoDeprecatedPatterns(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "new-style", `---
description: Uses metadata URL
mode: interactive
---
URL: {{index .Task.Meta "url"}}
`)

	lr, err := Load(dir, "new-style")
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.DeprecatedPatterns) != 0 {
		t.Errorf("DeprecatedPatterns = %v, want empty", lr.DeprecatedPatterns)
	}
}

func TestLoad_UnknownFields(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "extra", `---
description: Has extras
mode: interactive
allowed_tools: ["bash"]
timeout: 30
---
Body.
`)

	lr, err := Load(dir, "extra")
	if err != nil {
		t.Fatal(err)
	}

	if len(lr.UnknownFields) != 2 {
		t.Fatalf("UnknownFields = %v, want 2 fields", lr.UnknownFields)
	}
	if lr.UnknownFields[0] != "allowed_tools" {
		t.Errorf("UnknownFields[0] = %q, want %q", lr.UnknownFields[0], "allowed_tools")
	}
	if lr.UnknownFields[1] != "timeout" {
		t.Errorf("UnknownFields[1] = %q, want %q", lr.UnknownFields[1], "timeout")
	}
	if lr.Prompt.Config.Description != "Has extras" {
		t.Errorf("Description = %q, want %q", lr.Prompt.Config.Description, "Has extras")
	}
}

func TestLoad_NoUnknownFields(t *testing.T) {
	dir := t.TempDir()
	writePrompt(t, dir, "clean", `---
description: Clean prompt
mode: noninteractive
on_done: review
on_cancel: notify
---
Body.
`)

	lr, err := Load(dir, "clean")
	if err != nil {
		t.Fatal(err)
	}
	if len(lr.UnknownFields) != 0 {
		t.Errorf("UnknownFields = %v, want empty", lr.UnknownFields)
	}
}

func TestLoad_InternalPrompt(t *testing.T) {
	lr, err := Load("", "internal:remove-unknown-frontmatter")
	if err != nil {
		t.Fatal(err)
	}
	if lr.Prompt.Config.Mode != "interactive" {
		t.Errorf("Mode = %q, want %q", lr.Prompt.Config.Mode, "interactive")
	}
	if lr.Prompt.ID != "internal:remove-unknown-frontmatter" {
		t.Errorf("ID = %q, want %q", lr.Prompt.ID, "internal:remove-unknown-frontmatter")
	}
}

func TestRender_MissingMetaKey(t *testing.T) {
	p := &Prompt{
		ID:   "test",
		Body: `Instruction: {{index .Action.Meta "instruction"}}`,
	}

	data := PromptData{
		Task:    TaskData{Meta: map[string]any{}},
		Project: ProjectData{Meta: map[string]any{}},
		Action:  ActionData{Meta: map[string]any{}},
	}

	_, err := p.Render(data)
	if err == nil {
		t.Fatal("expected error for missing meta key")
	}
	if !strings.Contains(err.Error(), `missing metadata key "instruction"`) {
		t.Errorf("error = %q, want to contain 'missing metadata key \"instruction\"'", err.Error())
	}
}

func TestRender_MissingMetaKey_DotSyntax(t *testing.T) {
	p := &Prompt{
		ID:   "test",
		Body: `Instruction: {{.Action.Meta.instruction}}`,
	}

	data := PromptData{
		Task:    TaskData{Meta: map[string]any{}},
		Project: ProjectData{Meta: map[string]any{}},
		Action:  ActionData{Meta: map[string]any{}},
	}

	_, err := p.Render(data)
	if err == nil {
		t.Fatal("expected error for missing meta key via dot syntax")
	}
}

func TestRender_SoftIndex_MissingKey(t *testing.T) {
	p := &Prompt{
		ID:   "test",
		Body: `URL: {{get .Task.Meta "url"}}`,
	}
	data := PromptData{
		Task:    TaskData{Meta: map[string]any{}},
		Project: ProjectData{Meta: map[string]any{}},
		Action:  ActionData{Meta: map[string]any{}},
	}
	result, err := p.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if result != "URL: " {
		t.Errorf("Render = %q, want %q", result, "URL: ")
	}
}

func TestRender_SoftIndex_KeyExists(t *testing.T) {
	p := &Prompt{
		ID:   "test",
		Body: `URL: {{get .Task.Meta "url"}}`,
	}
	data := PromptData{
		Task:    TaskData{Meta: map[string]any{"url": "https://example.com"}},
		Project: ProjectData{Meta: map[string]any{}},
		Action:  ActionData{Meta: map[string]any{}},
	}
	result, err := p.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if result != "URL: https://example.com" {
		t.Errorf("Render = %q, want %q", result, "URL: https://example.com")
	}
}

func TestRender_SoftIndex_IfGuard(t *testing.T) {
	p := &Prompt{
		ID:   "test",
		Body: `{{if get .Task.Meta "url"}}URL: {{get .Task.Meta "url"}}{{end}}done`,
	}
	data := PromptData{
		Task:    TaskData{Meta: map[string]any{}},
		Project: ProjectData{Meta: map[string]any{}},
		Action:  ActionData{Meta: map[string]any{}},
	}
	result, err := p.Render(data)
	if err != nil {
		t.Fatal(err)
	}
	if result != "done" {
		t.Errorf("Render = %q, want %q", result, "done")
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
