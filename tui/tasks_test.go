package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func TestTasksModel_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	view := m.View()
	if !strings.Contains(view, "No tasks") {
		t.Errorf("empty view should show 'No tasks', got %q", view)
	}
}

func TestTasksModel_LoadAndExpand(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Fix bug", `{"url":"https://example.com"}`, "")
	d.InsertAction("check-pr", taskID, "{}", db.ActionStatusPending)
	d.InsertAction("fix-ci", taskID, "{}", db.ActionStatusRunning)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Default expanded: project, task, and actions should all be visible
	view := m.View()
	if !strings.Contains(view, "immedio") {
		t.Errorf("view should contain project name 'immedio', got %q", view)
	}
	if !strings.Contains(view, "Fix bug") {
		t.Errorf("view should show task 'Fix bug', got %q", view)
	}
	if !strings.Contains(view, "check-pr") {
		t.Errorf("view should show action 'check-pr', got %q", view)
	}
	if !strings.Contains(view, "fix-ci") {
		t.Errorf("view should show action 'fix-ci', got %q", view)
	}
}

func TestTasksModel_Navigation(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Task A", "{}", "")
	d.InsertAction("a", taskID1, "{}", db.ActionStatusPending)
	taskID2, _ := d.InsertTask(2, "Task B", "{}", "")
	d.InsertAction("b", taskID2, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// 3 projects + 2 tasks (open, expanded) + 2 actions = 7 lines
	if len(m.lines) != 7 {
		t.Errorf("lines = %d, want 7 (fully expanded)", len(m.lines))
	}

	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("after j, cursor = %d, want 1", m.cursor)
	}

	// Navigate to end
	for range 10 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	last := len(m.lines) - 1
	if m.cursor != last {
		t.Errorf("at end, cursor = %d, want %d", m.cursor, last)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != last-1 {
		t.Errorf("after k, cursor = %d, want %d", m.cursor, last-1)
	}
}

func TestTasksModel_CollapseExpand(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// 3 projects + 1 task (open, expanded) + 1 action = 5 lines
	if len(m.lines) != 5 {
		t.Fatalf("lines = %d, want 5", len(m.lines))
	}

	// Collapse first project (immedio)
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	// collapsed project 1 + project 2 + project 3 = 3 lines
	if len(m.lines) != 3 {
		t.Errorf("after collapse project, lines = %d, want 3", len(m.lines))
	}

	// Expand project again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.lines) != 5 {
		t.Errorf("after expand project, lines = %d, want 5", len(m.lines))
	}
}

func TestTasksModel_Reload(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// 3 seeded projects with no tasks = 3 lines
	if len(m.lines) != 3 {
		t.Errorf("initial lines = %d, want 3", len(m.lines))
	}

	taskID, _ := d.InsertTask(1, "New Task", "{}", "")
	d.InsertAction("x", taskID, "{}", db.ActionStatusPending)

	// Reload via loadTasks (auto-reload on tick)
	reloadMsg := m.loadTasks()()
	m, _ = m.Update(reloadMsg)

	// 3 projects + 1 task (open, expanded) + 1 action = 5 lines
	if len(m.lines) != 5 {
		t.Errorf("after reload, lines = %d, want 5", len(m.lines))
	}
}

func TestTasksModel_DateFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Today task", "{}", "")
	d.InsertAction("today-action", taskID1, "{}", db.ActionStatusPending)

	taskID2, _ := d.InsertTask(1, "Old task", "{}", "")
	d.InsertAction("old-action", taskID2, "{}", db.ActionStatusPending)
	d.UpdateTask(taskID2, db.TaskStatusDone, "")

	// Set old-action and old task dates to a different date
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE title ='old-action'")
	d.Exec(fmt.Sprintf("UPDATE tasks SET created_at = '2025-01-01 00:00:00', updated_at = '2025-01-01 00:00:00' WHERE id = %d", taskID2))

	// Get today's date from the first action
	actions, _ := d.ListActions("", nil, 0)
	var todayDate string
	for _, a := range actions {
		if a.Title == "today-action" {
			todayDate = a.CreatedAt[:10]
			break
		}
	}

	m := NewTasksModel(d, todayDate)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "Today task") {
		t.Errorf("view should contain 'Today task', got %q", view)
	}
	if strings.Contains(view, "Old task") {
		t.Errorf("view should not contain 'Old task', got %q", view)
	}

	// Without filter, both should appear
	m2 := NewTasksModel(d, "")
	msg2 := m2.Init()()
	m2, _ = m2.Update(msg2)

	view2 := m2.View()
	if !strings.Contains(view2, "Today task") {
		t.Errorf("unfiltered view should contain 'Today task', got %q", view2)
	}
	if !strings.Contains(view2, "Old task") {
		t.Errorf("unfiltered view should contain 'Old task', got %q", view2)
	}
}

func TestTasksModel_DateFilter_NonDoneTaskShown(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Open task with action on a different date
	taskID, _ := d.InsertTask(1, "Open no-match task", "{}", "")
	d.InsertAction("old-action", taskID, "{}", db.ActionStatusPending)
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE title ='old-action'")

	m := NewTasksModel(d, "2026-03-03")
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "Open no-match task") {
		t.Errorf("view should contain non-done task even with no matching actions, got %q", view)
	}
}

func TestTasksModel_DateFilter_ArchivedTaskFiltered(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Archived task with old dates — should be filtered out
	taskID, _ := d.InsertTask(1, "Old archived", "{}", "")
	d.InsertAction("old-action", taskID, "{}", db.ActionStatusPending)
	d.UpdateTask(taskID, db.TaskStatusArchived, "")
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE title ='old-action'")
	d.Exec(fmt.Sprintf("UPDATE tasks SET created_at = '2025-01-01 00:00:00', updated_at = '2025-01-01 00:00:00' WHERE id = %d", taskID))

	// Open task — should always appear
	taskID2, _ := d.InsertTask(1, "Open task", "{}", "")
	d.InsertAction("open-action", taskID2, "{}", db.ActionStatusPending)

	actions, _ := d.ListActions("", nil, 0)
	var todayDate string
	for _, a := range actions {
		if a.Title == "open-action" {
			todayDate = a.CreatedAt[:10]
			break
		}
	}

	m := NewTasksModel(d, todayDate)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if strings.Contains(view, "Old archived") {
		t.Errorf("archived task with old date should be filtered out, got %q", view)
	}
	if !strings.Contains(view, "Open task") {
		t.Errorf("open task should be visible, got %q", view)
	}
}

func TestTasksModel_ArchivedTaskCollapsed(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Archived task", "{}", "")
	d.InsertAction("check", taskID, "{}", db.ActionStatusPending)
	d.UpdateTask(taskID, db.TaskStatusArchived, "")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// 3 projects + 1 archived task (collapsed, action hidden) = 4 lines
	if len(m.lines) != 4 {
		t.Errorf("lines = %d, want 4 (archived task should be collapsed)", len(m.lines))
	}
}

func TestTasksModel_InlineResult(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	id, _ := d.InsertAction("check", taskID, "{}", db.ActionStatusRunning)
	d.MarkDone(id, "all passed")

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to the action line (project=0, task=1, action=2)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	view := m.View()
	if !strings.Contains(view, "result: all passed") {
		t.Errorf("view should contain inline result, got %q", view)
	}
}

func TestTasksModel_DetailView(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	id, _ := d.InsertAction("check", taskID, "{}", db.ActionStatusRunning)
	d.MarkDone(id, "detailed output\nline 2")

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to the action line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if m.mode != modeNormal {
		t.Fatalf("mode = %d, want modeNormal", m.mode)
	}

	// Press v to enter detail view
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.mode != modeViewDetail {
		t.Fatalf("mode = %d, want modeViewDetail", m.mode)
	}

	view := m.View()
	if !strings.Contains(view, "Action Detail") {
		t.Errorf("detail view should contain header, got %q", view)
	}
	if !strings.Contains(view, "detailed output") {
		t.Errorf("detail view should contain result, got %q", view)
	}

	// Press q to return
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after q", m.mode)
	}
}

func TestTasksModel_DetailViewNoResultNoOp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	d.InsertAction("check", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to action line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press v - should be no-op (no result)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.mode != modeNormal {
		t.Errorf("v on action with no result should be no-op, mode = %d", m.mode)
	}
}

func TestTasksModel_DetailViewOnProjectLineNoOp(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	id, _ := d.InsertAction("check", taskID, "{}", db.ActionStatusRunning)
	d.MarkDone(id, "some result")

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Cursor at 0 = project line (no action)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.mode != modeNormal {
		t.Errorf("v on project line should be no-op, mode = %d", m.mode)
	}
}

func TestTasksModel_VisibleRange_AllVisible(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	m = m.SetSize(80, 40) // height 40 → maxVisible=38, plenty of room
	msg := m.Init()()
	m, _ = m.Update(msg)

	vr := m.visibleRange()
	if vr.start != 0 || vr.end != len(m.lines) {
		t.Errorf("visibleRange = {%d, %d}, want {0, %d}", vr.start, vr.end, len(m.lines))
	}
}

func TestTasksModel_VisibleRange_Scroll(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Create enough lines to exceed viewport: 1 project + 30 tasks + 30 actions = 61 lines
	for i := range 30 {
		taskID, _ := d.InsertTask(1, fmt.Sprintf("Task %d", i), "{}", "")
		d.InsertAction("a", taskID, "{}", db.ActionStatusPending)
	}

	m := NewTasksModel(d, "")
	m = m.SetSize(80, 12) // height 12 → maxVisible=9 (headerRows=3 for summary line)
	msg := m.Init()()
	m, _ = m.Update(msg)

	if len(m.lines) <= 9 {
		t.Fatalf("need more than 9 lines for scroll test, got %d", len(m.lines))
	}

	// Cursor at 0: start should be 0
	vr := m.visibleRange()
	if vr.start != 0 {
		t.Errorf("cursor=0: start = %d, want 0", vr.start)
	}
	if vr.end-vr.start != 9 {
		t.Errorf("cursor=0: window size = %d, want 9", vr.end-vr.start)
	}

	// Move cursor to middle
	for range 20 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	if m.cursor != 20 {
		t.Fatalf("cursor = %d, want 20", m.cursor)
	}

	vr = m.visibleRange()
	if vr.start > m.cursor || vr.end <= m.cursor {
		t.Errorf("cursor=%d not in visible range {%d, %d}", m.cursor, vr.start, vr.end)
	}

	// Move cursor to end
	for i := 0; i < len(m.lines); i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	lastIdx := len(m.lines) - 1
	if m.cursor != lastIdx {
		t.Fatalf("cursor = %d, want %d", m.cursor, lastIdx)
	}

	vr = m.visibleRange()
	if vr.end != len(m.lines) {
		t.Errorf("cursor at end: end = %d, want %d", vr.end, len(m.lines))
	}
	if vr.end-vr.start != 9 {
		t.Errorf("cursor at end: window size = %d, want 9", vr.end-vr.start)
	}
}

func TestTasksModel_SortOrder(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Create tasks in order: open(1), archived(2), done(3)
	d.InsertTask(1, "TaskA-open", "{}", "")
	taskID2, _ := d.InsertTask(1, "TaskB-archived", "{}", "")
	d.UpdateTask(taskID2, db.TaskStatusArchived, "")
	taskID3, _ := d.InsertTask(1, "TaskC-done", "{}", "")
	d.UpdateTask(taskID3, db.TaskStatusDone, "")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Expected sort: done(3) → open(1) → archived(2)
	if len(m.trees) != 3 {
		t.Fatalf("trees = %d, want 3", len(m.trees))
	}
	// All tasks are in project 1 (immedio)
	tasks := m.trees[0].tasks
	if len(tasks) != 3 {
		t.Fatalf("tasks = %d, want 3", len(tasks))
	}
	if tasks[0].task.Status != db.TaskStatusDone {
		t.Errorf("tasks[0].status = %q, want %q", tasks[0].task.Status, db.TaskStatusDone)
	}
	if tasks[1].task.Status != db.TaskStatusOpen {
		t.Errorf("tasks[1].status = %q, want %q", tasks[1].task.Status, db.TaskStatusOpen)
	}
	if tasks[2].task.Status != db.TaskStatusArchived {
		t.Errorf("tasks[2].status = %q, want %q", tasks[2].task.Status, db.TaskStatusArchived)
	}
}

func TestTasksModel_DisabledProjectDisplay(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable "works" project (id=3)
	d.SetDispatchEnabled(3, false)

	taskID, _ := d.InsertTask(3, "Works task", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "⊘") {
		t.Errorf("disabled project should show ⊘ icon, got %q", view)
	}
	if !strings.Contains(view, "works") {
		t.Errorf("disabled project name should still be shown, got %q", view)
	}
}

func TestTasksModel_ToggleFocus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Cursor should be on the first project line
	if m.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", m.cursor)
	}

	// Press f to toggle focus (disable)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if cmd != nil {
		reloadMsg := cmd()
		m, _ = m.Update(reloadMsg)
	}

	// Verify project is now disabled
	p, _ := d.GetProjectByID(1)
	if p.DispatchEnabled {
		t.Error("expected project to be disabled after toggle")
	}

	view := m.View()
	if !strings.Contains(view, "⊘") {
		t.Errorf("disabled project should show ⊘, got %q", view)
	}
}

func TestTasksModel_ToggleFocusOnlyOnProjectLine(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Move to task line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press f on task line — should be a no-op
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	if cmd != nil {
		t.Error("f on non-project line should be no-op")
	}

	// Project should still be enabled
	p, _ := d.GetProjectByID(1)
	if !p.DispatchEnabled {
		t.Error("project should still be enabled")
	}
}

func TestTasksModel_ProjectsWithoutTasks(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// No tasks — all 3 projects should still appear for focus toggling
	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	if len(m.trees) != 3 {
		t.Errorf("trees = %d, want 3 (all projects)", len(m.trees))
	}
}

func TestTasksModel_DetailViewEscBack(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	id, _ := d.InsertAction("check", taskID, "{}", db.ActionStatusRunning)
	d.MarkDone(id, "some result")

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to action line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Enter detail view
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.mode != modeViewDetail {
		t.Fatal("should be in detail view")
	}

	// Press esc — should return to normal mode
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Error("esc should return to normal mode from detail view")
	}
}

func TestTasksModel_SetSize(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	m = m.SetSize(100, 50)

	if m.width != 100 || m.height != 50 {
		t.Errorf("size = %dx%d, want 100x50", m.width, m.height)
	}
}

func TestTasksModel_SummaryLine(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "{}", "")
	d.InsertAction("a1", taskID, "{}", db.ActionStatusRunning)
	d.InsertAction("a2", taskID, "{}", db.ActionStatusPending)
	id3, _ := d.InsertAction("a3", taskID, "{}", db.ActionStatusRunning)
	d.MarkDone(id3, "ok")
	id4, _ := d.InsertAction("a4", taskID, "{}", db.ActionStatusRunning)
	d.MarkFailed(id4, "err")

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "1 running") {
		t.Errorf("summary should show '1 running', got %q", view)
	}
	if !strings.Contains(view, "1 pending") {
		t.Errorf("summary should show '1 pending', got %q", view)
	}
	if !strings.Contains(view, "1 done") {
		t.Errorf("summary should show '1 done', got %q", view)
	}
	if !strings.Contains(view, "1 failed") {
		t.Errorf("summary should show '1 failed', got %q", view)
	}
}

func TestTasksModel_SummaryLineUnfocused(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Task1", "{}", "")
	d.InsertAction("a1", taskID1, "{}", db.ActionStatusPending)
	taskID2, _ := d.InsertTask(2, "Task2", "{}", "")
	d.InsertAction("a2", taskID2, "{}", db.ActionStatusPending)
	d.InsertAction("a3", taskID2, "{}", db.ActionStatusPending)
	d.SetDispatchEnabled(2, false)

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "1 pending") {
		t.Errorf("summary should show '1 pending', got %q", view)
	}
	if !strings.Contains(view, "(2 unfocused)") {
		t.Errorf("summary should show '(2 unfocused)', got %q", view)
	}
}

func TestTasksModel_ProjectWorkDir(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "{}", "")
	d.InsertAction("a", taskID, "{}", db.ActionStatusPending)

	m := NewTasksModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !strings.Contains(view, "~/ghq/github.com/immedioinc/immedio") {
		t.Errorf("project line should show work_dir, got %q", view)
	}
}
