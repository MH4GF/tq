package view

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MH4GF/tq/testutil"
)

func TestGenerate_WithData(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}

	openTaskID, err := d.InsertTask(p.ID, "Implement feature X", "https://github.com/example/pr/1", "{}")
	if err != nil {
		t.Fatal(err)
	}

	doneTaskID, err := d.InsertTask(p.ID, "Fix bug Y", "", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTask(doneTaskID, "done"); err != nil {
		t.Fatal(err)
	}

	blockedTaskID, err := d.InsertTask(p.ID, "Blocked task Z", "", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTask(blockedTaskID, "blocked"); err != nil {
		t.Fatal(err)
	}

	actionID1, err := d.InsertAction("review-pr", &openTaskID, "{}", "done", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MarkDone(actionID1, "ok"); err != nil {
		t.Fatal(err)
	}

	if _, err := d.InsertAction("run-tests", &openTaskID, "{}", "running", 0, "auto"); err != nil {
		t.Fatal(err)
	}

	if _, err := d.InsertAction("deploy", &openTaskID, "{}", "pending", 0, "auto"); err != nil {
		t.Fatal(err)
	}

	failedActionID, err := d.InsertAction("lint", &blockedTaskID, "{}", "failed", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MarkFailed(failedActionID, "lint errors"); err != nil {
		t.Fatal(err)
	}

	_, err = d.InsertAction("approve", &blockedTaskID, "{}", "waiting_human", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "### immedio") {
		t.Error("expected project header '### immedio'")
	}

	inProgressIdx := strings.Index(result, "#### In Progress")
	doneIdx := strings.Index(result, "#### Done")
	blockedIdx := strings.Index(result, "#### Blocked")

	if inProgressIdx == -1 {
		t.Fatal("expected '#### In Progress' section")
	}
	if doneIdx == -1 {
		t.Fatal("expected '#### Done' section")
	}
	if blockedIdx == -1 {
		t.Fatal("expected '#### Blocked' section")
	}
	if doneIdx >= inProgressIdx {
		t.Error("Done should come before In Progress")
	}
	if inProgressIdx >= blockedIdx {
		t.Error("In Progress should come before Blocked")
	}

	if !strings.Contains(result, fmt.Sprintf("- #%d Implement feature X [link](https://github.com/example/pr/1)", openTaskID)) {
		t.Errorf("expected open task with link, got:\n%s", result)
	}
	if !strings.Contains(result, fmt.Sprintf("- #%d Fix bug Y", doneTaskID)) {
		t.Errorf("expected done task, got:\n%s", result)
	}

	if !strings.Contains(result, "  - [x] review-pr") {
		t.Errorf("expected done action with [x], got:\n%s", result)
	}
	if !strings.Contains(result, "  - [ ] run-tests (running)") {
		t.Errorf("expected running action, got:\n%s", result)
	}
	if !strings.Contains(result, "  - [ ] deploy") {
		t.Errorf("expected pending action, got:\n%s", result)
	}
	if !strings.Contains(result, "  - [ ] lint (failed)") {
		t.Errorf("expected failed action, got:\n%s", result)
	}
	if !strings.Contains(result, "  - [ ] approve (waiting)") {
		t.Errorf("expected waiting action, got:\n%s", result)
	}
}

func TestGenerate_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	result, err := Generate(d, "")
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Errorf("expected empty string for no tasks, got: %q", result)
	}
}

func TestGenerate_EmptyProject(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.InsertTask(p.ID, "Some task", "", "{}"); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "### immedio") {
		t.Error("expected immedio project section")
	}
	if strings.Contains(result, "### hearable") {
		t.Error("expected hearable project to be absent")
	}
	if strings.Contains(result, "### works") {
		t.Error("expected works project to be absent")
	}
}

func TestGenerate_MultipleProjects(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p1, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}
	p2, err := d.GetProjectByName("hearable")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := d.InsertTask(p1.ID, "Task A", "", "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.InsertTask(p2.ID, "Task B", "", "{}"); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "### immedio") {
		t.Error("expected immedio section")
	}
	if !strings.Contains(result, "### hearable") {
		t.Error("expected hearable section")
	}
	if !strings.Contains(result, "Task A") {
		t.Error("expected Task A")
	}
	if !strings.Contains(result, "Task B") {
		t.Error("expected Task B")
	}
}

func TestGenerate_DateFilter(t *testing.T) {
	tests := []struct {
		name       string
		dateFilter string
		updateSQL  string
		wantTask   bool
	}{
		{
			name:       "match by created_at",
			dateFilter: "2026-01-15",
			updateSQL:  "UPDATE actions SET created_at = '2026-01-15 10:00:00'",
			wantTask:   true,
		},
		{
			name:       "match by started_at",
			dateFilter: "2026-02-20",
			updateSQL:  "UPDATE actions SET created_at = '2026-01-01 00:00:00', started_at = '2026-02-20 09:00:00'",
			wantTask:   true,
		},
		{
			name:       "match by completed_at",
			dateFilter: "2026-03-01",
			updateSQL:  "UPDATE actions SET created_at = '2026-01-01 00:00:00', completed_at = '2026-03-01 18:00:00'",
			wantTask:   true,
		},
		{
			name:       "no match but open task still shown",
			dateFilter: "2026-12-25",
			updateSQL:  "UPDATE actions SET created_at = '2026-01-01 00:00:00'",
			wantTask:   true,
		},
		{
			name:       "empty filter shows all",
			dateFilter: "",
			updateSQL:  "",
			wantTask:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			p, err := d.GetProjectByName("immedio")
			if err != nil {
				t.Fatal(err)
			}

			taskID, err := d.InsertTask(p.ID, "Test task", "", "{}")
			if err != nil {
				t.Fatal(err)
			}

			if _, err := d.InsertAction("review-pr", &taskID, "{}", "pending", 0, "auto"); err != nil {
				t.Fatal(err)
			}

			if tt.updateSQL != "" {
				if _, err := d.Exec(tt.updateSQL); err != nil {
					t.Fatal(err)
				}
			}

			result, err := Generate(d, tt.dateFilter)
			if err != nil {
				t.Fatal(err)
			}

			hasTask := strings.Contains(result, "Test task")
			if hasTask != tt.wantTask {
				t.Errorf("wantTask=%v, got result:\n%s", tt.wantTask, result)
			}
		})
	}
}

func TestGenerate_DateFilter_TaskWithNoActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := d.InsertTask(p.ID, "Task without actions", "", "{}"); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "2026-01-15")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Task without actions") {
		t.Errorf("open task without actions should be shown when date filter is set, got:\n%s", result)
	}
}

func TestGenerate_DateFilter_DoneTaskExcluded(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}

	taskID, err := d.InsertTask(p.ID, "Done task no match", "", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTask(taskID, "done"); err != nil {
		t.Fatal(err)
	}

	if _, err := d.InsertAction("review-pr", &taskID, "{}", "pending", 0, "auto"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Exec("UPDATE actions SET created_at = '2026-01-01 00:00:00'"); err != nil {
		t.Fatal(err)
	}
	// Task itself also has old dates
	if _, err := d.Exec("UPDATE tasks SET created_at = '2026-01-01 00:00:00', updated_at = '2026-01-01 00:00:00'"); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "2026-12-25")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, "Done task no match") {
		t.Errorf("done task with no matching actions and no matching task dates should be excluded, got:\n%s", result)
	}
}

func TestGenerate_DateFilter_DoneTaskShownByTaskDate(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}

	taskID, err := d.InsertTask(p.ID, "Done today no actions", "", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateTask(taskID, "done"); err != nil {
		t.Fatal(err)
	}
	// updated_at is set to now by UpdateTask, set it to our target date
	if _, err := d.Exec(fmt.Sprintf("UPDATE tasks SET updated_at = '2026-03-04 15:00:00' WHERE id = %d", taskID)); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "2026-03-04")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Done today no actions") {
		t.Errorf("done task updated today should be shown even with 0 matching actions, got:\n%s", result)
	}
}

func TestGenerate_DateFilter_OpenTaskShowsActiveActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}

	taskID, err := d.InsertTask(p.ID, "Open task with old actions", "", "{}")
	if err != nil {
		t.Fatal(err)
	}

	// pending action with old date
	if _, err := d.InsertAction("review-pr", &taskID, "{}", "pending", 0, "auto"); err != nil {
		t.Fatal(err)
	}
	// running action with old date
	if _, err := d.InsertAction("run-tests", &taskID, "{}", "running", 0, "auto"); err != nil {
		t.Fatal(err)
	}
	// waiting_human action with old date
	if _, err := d.InsertAction("approve", &taskID, "{}", "waiting_human", 0, "auto"); err != nil {
		t.Fatal(err)
	}

	// Set all actions to old date
	if _, err := d.Exec("UPDATE actions SET created_at = '2026-01-01 00:00:00'"); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "2026-12-25")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Open task with old actions") {
		t.Errorf("open task should be shown, got:\n%s", result)
	}
	if !strings.Contains(result, "review-pr") {
		t.Errorf("pending action should be shown regardless of date, got:\n%s", result)
	}
	if !strings.Contains(result, "run-tests") {
		t.Errorf("running action should be shown regardless of date, got:\n%s", result)
	}
	if !strings.Contains(result, "approve") {
		t.Errorf("waiting_human action should be shown regardless of date, got:\n%s", result)
	}
}

func TestGenerate_DateFilter_OpenTaskFiltersDoneActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	p, err := d.GetProjectByName("immedio")
	if err != nil {
		t.Fatal(err)
	}

	taskID, err := d.InsertTask(p.ID, "Open task mixed actions", "", "{}")
	if err != nil {
		t.Fatal(err)
	}

	// pending action (should always show)
	if _, err := d.InsertAction("deploy", &taskID, "{}", "pending", 0, "auto"); err != nil {
		t.Fatal(err)
	}
	// done action with old date (should be filtered out)
	doneID, err := d.InsertAction("review-pr", &taskID, "{}", "done", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if err := d.MarkDone(doneID, "ok"); err != nil {
		t.Fatal(err)
	}

	// Set all actions to old date
	if _, err := d.Exec("UPDATE actions SET created_at = '2026-01-01 00:00:00', completed_at = CASE WHEN completed_at IS NOT NULL THEN '2026-01-01 12:00:00' ELSE NULL END"); err != nil {
		t.Fatal(err)
	}

	result, err := Generate(d, "2026-12-25")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Open task mixed actions") {
		t.Errorf("open task should be shown, got:\n%s", result)
	}
	if !strings.Contains(result, "deploy") {
		t.Errorf("pending action should be shown, got:\n%s", result)
	}
	if strings.Contains(result, "review-pr") {
		t.Errorf("done action with non-matching date should be filtered out, got:\n%s", result)
	}
}

func TestInject(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily.md")

	original := "# Daily Note\nSome content before\n<!-- tq:start -->\nold content here\n<!-- tq:end -->\nSome content after\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	newContent := "### immedio\n#### In Progress\n- #1 Task\n"
	if err := Inject(path, newContent); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)

	if !strings.Contains(result, "# Daily Note\nSome content before\n") {
		t.Errorf("content before markers not preserved:\n%s", result)
	}

	if !strings.Contains(result, "Some content after") {
		t.Errorf("content after markers not preserved:\n%s", result)
	}

	if !strings.Contains(result, "<!-- tq:start -->\n### immedio\n#### In Progress\n- #1 Task\n<!-- tq:end -->") {
		t.Errorf("new content not injected correctly:\n%s", result)
	}
}

func TestInject_NoMarkers(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily.md")

	if err := os.WriteFile(path, []byte("# No markers here\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Inject(path, "content")
	if err == nil {
		t.Fatal("expected error for missing markers")
	}
	if !strings.Contains(err.Error(), "markers") {
		t.Errorf("expected error mentioning markers, got: %v", err)
	}
}

func TestInject_EmptyContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily.md")

	original := "before\n<!-- tq:start -->\nold stuff\n<!-- tq:end -->\nafter\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Inject(path, ""); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := string(data)

	if !strings.Contains(result, "<!-- tq:start -->\n<!-- tq:end -->") {
		t.Errorf("expected empty content between markers, got:\n%s", result)
	}
	if !strings.Contains(result, "before\n") {
		t.Errorf("content before markers not preserved:\n%s", result)
	}
	if !strings.Contains(result, "after") {
		t.Errorf("content after markers not preserved:\n%s", result)
	}
}
