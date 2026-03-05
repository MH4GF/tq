package cmd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/testutil"
)

func TestView_Print(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "fix login bug", "https://example.com/pr/1", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", "auto")

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"view"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "fix login bug") {
		t.Errorf("output should contain task title, got %q", out)
	}
	if !contains(out, "review-pr") {
		t.Errorf("output should contain action template, got %q", out)
	}
	if !contains(out, "immedio") {
		t.Errorf("output should contain project name, got %q", out)
	}
}

func TestView_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"view"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "no tasks") {
		t.Errorf("output = %q, want 'no tasks'", out)
	}
}

func TestView_Inject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	taskID, _ := d.InsertTask(1, "fix login bug", "", "{}")
	d.InsertAction("review-pr", &taskID, "{}", "pending", "auto")

	tmpDir := t.TempDir()
	cmd.SetTQDir(filepath.Join(tmpDir, "tq"))

	dailyDir := filepath.Join(tmpDir, "daily")
	if err := os.MkdirAll(dailyDir, 0755); err != nil {
		t.Fatal(err)
	}

	dailyPath := filepath.Join(dailyDir, "2026-02-27.md")
	initialContent := "# 2026-02-27\n\n## Tasks\n<!-- tq:start -->\nold content\n<!-- tq:end -->\n\n## Notes\nsome notes\n"
	if err := os.WriteFile(dailyPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"view", "--inject", "--all", "--date", "2026-02-27"})

	if err := root.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !contains(out, "injected into") {
		t.Errorf("output should confirm injection, got %q", out)
	}

	data, err := os.ReadFile(dailyPath)
	if err != nil {
		t.Fatal(err)
	}
	fileContent := string(data)

	if !strings.Contains(fileContent, "fix login bug") {
		t.Errorf("file should contain task title, got %q", fileContent)
	}
	if !strings.Contains(fileContent, "review-pr") {
		t.Errorf("file should contain action template, got %q", fileContent)
	}
	if !strings.Contains(fileContent, "<!-- tq:start -->") {
		t.Errorf("file should preserve start marker, got %q", fileContent)
	}
	if !strings.Contains(fileContent, "<!-- tq:end -->") {
		t.Errorf("file should preserve end marker, got %q", fileContent)
	}
	if strings.Contains(fileContent, "old content") {
		t.Errorf("file should not contain old content, got %q", fileContent)
	}
	if !strings.Contains(fileContent, "some notes") {
		t.Errorf("file should preserve content outside markers, got %q", fileContent)
	}
}

func TestView_Inject_NoFile(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()

	tmpDir := t.TempDir()
	cmd.SetTQDir(filepath.Join(tmpDir, "tq"))

	root := cmd.GetRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"view", "--inject", "--date", "9999-01-01"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing daily note file")
	}
	if !contains(err.Error(), "daily note not found") {
		t.Errorf("error = %q, want to contain 'daily note not found'", err.Error())
	}
}
