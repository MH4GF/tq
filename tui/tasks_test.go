package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/testutil"
)

func TestTasksModel_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	view := m.View()
	if !contains(view, "No tasks") {
		t.Errorf("empty view should show 'No tasks', got %q", view)
	}
}

func TestTasksModel_LoadAndExpand(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Fix bug", "https://example.com", "{}")
	d.InsertAction("check-pr", &taskID, "{}", "pending", "auto")
	d.InsertAction("fix-ci", &taskID, "{}", "running", "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Default expanded: project and task should be visible
	view := m.View()
	if !contains(view, "immedio") {
		t.Errorf("view should contain project name 'immedio', got %q", view)
	}
	if !contains(view, "Fix bug") {
		t.Errorf("view should show task 'Fix bug', got %q", view)
	}
	// Actions are no longer shown in tasks view (they are in queue view)
	if !contains(view, "2 actions") {
		t.Errorf("view should show action count '2 actions', got %q", view)
	}
}

func TestTasksModel_Navigation(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Task A", "", "{}")
	d.InsertAction("a", &taskID1, "{}", "pending", "auto")
	taskID2, _ := d.InsertTask(2, "Task B", "", "{}")
	d.InsertAction("b", &taskID2, "{}", "pending", "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// 3 projects (seeded) + 2 tasks = 5 lines (no action lines)
	if len(m.lines) != 5 {
		t.Errorf("lines = %d, want 5 (projects + tasks, no actions)", len(m.lines))
	}

	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("after j, cursor = %d, want 1", m.cursor)
	}

	// Navigate to end
	for i := 0; i < 10; i++ {
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

	taskID, _ := d.InsertTask(1, "Task", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// 3 projects + 1 task = 4 lines (no action lines)
	if len(m.lines) != 4 {
		t.Fatalf("lines = %d, want 4", len(m.lines))
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
	if len(m.lines) != 4 {
		t.Errorf("after expand project, lines = %d, want 4", len(m.lines))
	}
}

func TestTasksModel_SelectTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Fix bug", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to task line (line 1)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press enter on task line should emit taskSelectedMsg
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on task line should return a command")
	}

	selMsg := cmd()
	tsm, ok := selMsg.(taskSelectedMsg)
	if !ok {
		t.Fatalf("expected taskSelectedMsg, got %T", selMsg)
	}
	if tsm.taskID != taskID {
		t.Errorf("taskSelectedMsg.taskID = %d, want %d", tsm.taskID, taskID)
	}
	if tsm.title != "Fix bug" {
		t.Errorf("taskSelectedMsg.title = %q, want %q", tsm.title, "Fix bug")
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

	taskID, _ := d.InsertTask(1, "New Task", "", "{}")
	d.InsertAction("x", &taskID, "{}", "pending", "auto")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		reloadMsg := cmd()
		m, _ = m.Update(reloadMsg)
	}

	// 3 projects + 1 task = 4 lines (no action lines)
	if len(m.lines) != 4 {
		t.Errorf("after reload, lines = %d, want 4", len(m.lines))
	}
}

func TestTasksModel_DateFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Today task", "", "{}")
	d.InsertAction("today-action", &taskID1, "{}", "pending", "auto")

	taskID2, _ := d.InsertTask(1, "Old task", "", "{}")
	d.InsertAction("old-action", &taskID2, "{}", "pending", "auto")
	d.UpdateTask(taskID2, "done")

	// Set old-action and old task dates to a different date
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE prompt_id = 'old-action'")
	d.Exec(fmt.Sprintf("UPDATE tasks SET created_at = '2025-01-01 00:00:00', updated_at = '2025-01-01 00:00:00' WHERE id = %d", taskID2))

	// Get today's date from the first action
	actions, _ := d.ListActions("", nil)
	var todayDate string
	for _, a := range actions {
		if a.PromptID == "today-action" {
			todayDate = a.CreatedAt[:10]
			break
		}
	}

	m := NewTasksModel(d, todayDate)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "Today task") {
		t.Errorf("view should contain 'Today task', got %q", view)
	}
	if contains(view, "Old task") {
		t.Errorf("view should not contain 'Old task', got %q", view)
	}

	// Without filter, both should appear
	m2 := NewTasksModel(d, "")
	msg2 := m2.Init()()
	m2, _ = m2.Update(msg2)

	view2 := m2.View()
	if !contains(view2, "Today task") {
		t.Errorf("unfiltered view should contain 'Today task', got %q", view2)
	}
	if !contains(view2, "Old task") {
		t.Errorf("unfiltered view should contain 'Old task', got %q", view2)
	}
}

func TestTasksModel_DateFilter_NonDoneTaskShown(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Open task with action on a different date
	taskID, _ := d.InsertTask(1, "Open no-match task", "", "{}")
	d.InsertAction("old-action", &taskID, "{}", "pending", "auto")
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE prompt_id = 'old-action'")

	m := NewTasksModel(d, "2026-03-03")
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "Open no-match task") {
		t.Errorf("view should contain non-done task even with no matching actions, got %q", view)
	}
}

func TestTasksModel_DateFilter_ArchivedTaskFiltered(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Archived task with old dates — should be filtered out
	taskID, _ := d.InsertTask(1, "Old archived", "", "{}")
	d.InsertAction("old-action", &taskID, "{}", "pending", "auto")
	d.UpdateTask(taskID, "archived")
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE prompt_id = 'old-action'")
	d.Exec(fmt.Sprintf("UPDATE tasks SET created_at = '2025-01-01 00:00:00', updated_at = '2025-01-01 00:00:00' WHERE id = %d", taskID))

	// Open task — should always appear
	taskID2, _ := d.InsertTask(1, "Open task", "", "{}")
	d.InsertAction("open-action", &taskID2, "{}", "pending", "auto")

	actions, _ := d.ListActions("", nil)
	var todayDate string
	for _, a := range actions {
		if a.PromptID == "open-action" {
			todayDate = a.CreatedAt[:10]
			break
		}
	}

	m := NewTasksModel(d, todayDate)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if contains(view, "Old archived") {
		t.Errorf("archived task with old date should be filtered out, got %q", view)
	}
	if !contains(view, "Open task") {
		t.Errorf("open task should be visible, got %q", view)
	}
}

func TestTasksModel_ActionCount(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task with actions", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", "auto")
	d.InsertAction("fix", &taskID, "{}", "running", "auto")
	d.InsertAction("deploy", &taskID, "{}", "done", "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "3 actions") {
		t.Errorf("view should show '3 actions', got %q", view)
	}
}

func TestTasksModel_VisibleRange_AllVisible(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", "auto")

	m := NewTasksModel(d, "")
	m = m.SetSize(80, 40)
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

	// Create enough lines to exceed viewport: 1 project + 30 tasks = 31 lines
	for i := 0; i < 30; i++ {
		taskID, _ := d.InsertTask(1, fmt.Sprintf("Task %d", i), "", "{}")
		d.InsertAction("a", &taskID, "{}", "pending", "auto")
	}

	m := NewTasksModel(d, "")
	m = m.SetSize(80, 12) // height 12 → maxVisible=10
	msg := m.Init()()
	m, _ = m.Update(msg)

	if len(m.lines) <= 10 {
		t.Fatalf("need more than 10 lines for scroll test, got %d", len(m.lines))
	}

	// Cursor at 0: start should be 0
	vr := m.visibleRange()
	if vr.start != 0 {
		t.Errorf("cursor=0: start = %d, want 0", vr.start)
	}
	if vr.end-vr.start != 10 {
		t.Errorf("cursor=0: window size = %d, want 10", vr.end-vr.start)
	}

	// Move cursor to middle
	for i := 0; i < 20; i++ {
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
	if vr.end-vr.start != 10 {
		t.Errorf("cursor at end: window size = %d, want 10", vr.end-vr.start)
	}
}

func TestTasksModel_SortOrder(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Create tasks in order: open(1), archived(2), done(3)
	d.InsertTask(1, "TaskA-open", "", "{}")
	taskID2, _ := d.InsertTask(1, "TaskB-archived", "", "{}")
	d.UpdateTask(taskID2, "archived")
	taskID3, _ := d.InsertTask(1, "TaskC-done", "", "{}")
	d.UpdateTask(taskID3, "done")

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
	if tasks[0].task.Status != "done" {
		t.Errorf("tasks[0].status = %q, want done", tasks[0].task.Status)
	}
	if tasks[1].task.Status != "open" {
		t.Errorf("tasks[1].status = %q, want open", tasks[1].task.Status)
	}
	if tasks[2].task.Status != "archived" {
		t.Errorf("tasks[2].status = %q, want archived", tasks[2].task.Status)
	}
}

func TestTasksModel_DisabledProjectDisplay(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	// Disable "works" project (id=3)
	d.SetDispatchEnabled(3, false)

	taskID, _ := d.InsertTask(3, "Works task", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "⊘") {
		t.Errorf("disabled project should show ⊘ icon, got %q", view)
	}
	if !contains(view, "works") {
		t.Errorf("disabled project name should still be shown, got %q", view)
	}
}

func TestTasksModel_ToggleFocus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", "auto")

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
	if !contains(view, "⊘") {
		t.Errorf("disabled project should show ⊘, got %q", view)
	}
}

func TestTasksModel_ToggleFocusOnlyOnProjectLine(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", "auto")

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

func TestTasksModel_SetSize(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	m = m.SetSize(100, 50)

	if m.width != 100 || m.height != 50 {
		t.Errorf("size = %dx%d, want 100x50", m.width, m.height)
	}
}
