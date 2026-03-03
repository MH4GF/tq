package tui

import (
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
	d.InsertAction("check-pr", &taskID, "{}", "pending", 5, "auto")
	d.InsertAction("fix-ci", &taskID, "{}", "running", 3, "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Default expanded: project, task, and actions should all be visible
	view := m.View()
	if !contains(view, "immedio") {
		t.Errorf("view should contain project name 'immedio', got %q", view)
	}
	if !contains(view, "Fix bug") {
		t.Errorf("view should show task 'Fix bug', got %q", view)
	}
	if !contains(view, "check-pr") {
		t.Errorf("view should show action 'check-pr', got %q", view)
	}
	if !contains(view, "fix-ci") {
		t.Errorf("view should show action 'fix-ci', got %q", view)
	}
}

func TestTasksModel_Navigation(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Task A", "", "{}")
	d.InsertAction("a", &taskID1, "{}", "pending", 0, "auto")
	taskID2, _ := d.InsertTask(2, "Task B", "", "{}")
	d.InsertAction("b", &taskID2, "{}", "pending", 0, "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Default expanded: 2 projects + 2 tasks + 2 actions = 6 lines
	if len(m.lines) != 6 {
		t.Errorf("lines = %d, want 6 (fully expanded)", len(m.lines))
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
	if m.cursor != 5 {
		t.Errorf("at end, cursor = %d, want 5", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 4 {
		t.Errorf("after k, cursor = %d, want 4", m.cursor)
	}
}

func TestTasksModel_CollapseExpand(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Task", "", "{}")
	d.InsertAction("a", &taskID, "{}", "pending", 0, "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Default expanded: project + task + action = 3 lines
	if len(m.lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(m.lines))
	}

	// Collapse project
	m.cursor = 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.lines) != 1 {
		t.Errorf("after collapse project, lines = %d, want 1", len(m.lines))
	}

	// Expand project again
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.lines) != 3 {
		t.Errorf("after expand project, lines = %d, want 3", len(m.lines))
	}
}

func TestTasksModel_Reload(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	if len(m.lines) != 0 {
		t.Errorf("initial lines = %d, want 0", len(m.lines))
	}

	taskID, _ := d.InsertTask(1, "New Task", "", "{}")
	d.InsertAction("x", &taskID, "{}", "pending", 0, "auto")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		reloadMsg := cmd()
		m, _ = m.Update(reloadMsg)
	}

	// Default expanded: project + task + action = 3 lines
	if len(m.lines) != 3 {
		t.Errorf("after reload, lines = %d, want 3", len(m.lines))
	}
}

func TestTasksModel_CreateTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Press 'c' to start task creation
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if m.mode != modePickProject {
		t.Fatalf("mode = %d, want modePickProject(%d)", m.mode, modePickProject)
	}

	view := m.View()
	if !contains(view, "select project") {
		t.Errorf("view should show project selection, got %q", view)
	}

	// Select first project (enter)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeInputTitle {
		t.Fatalf("mode = %d, want modeInputTitle(%d)", m.mode, modeInputTitle)
	}

	// Type title
	for _, r := range "Test Task" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeInputURL {
		t.Fatalf("mode = %d, want modeInputURL(%d)", m.mode, modeInputURL)
	}

	// Skip URL (empty enter)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeNormal {
		t.Fatalf("mode = %d, want modeNormal(%d)", m.mode, modeNormal)
	}
	if cmd == nil {
		t.Fatal("expected createTask cmd")
	}

	// Execute the cmd
	result := cmd()
	m, _ = m.Update(result)

	if !contains(m.message, "task #") {
		t.Errorf("message = %q, want to contain 'task #'", m.message)
	}
}

func TestTasksModel_CreateTaskCancel(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Press 'c' then esc
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after esc", m.mode)
	}
}

func TestTasksModel_ChangeStatus(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Fix bug", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", 0, "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to task line (cursor 0 = project, cursor 1 = task)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press 's' to open status picker
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.mode != modePickStatus {
		t.Fatalf("mode = %d, want modePickStatus(%d)", m.mode, modePickStatus)
	}

	// Current status is "open", so it should not be in the list
	for _, s := range m.statuses {
		if s == "open" {
			t.Error("statuses should not contain current status 'open'")
		}
	}

	// Select "done" — navigate to it
	doneIdx := -1
	for i, s := range m.statuses {
		if s == "done" {
			doneIdx = i
			break
		}
	}
	if doneIdx < 0 {
		t.Fatal("'done' not found in statuses")
	}
	for i := 0; i < doneIdx; i++ {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}

	// Confirm
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected updateTaskStatus cmd")
	}

	result := cmd()
	m, _ = m.Update(result)

	if !contains(m.message, "done") {
		t.Errorf("message = %q, want to contain 'done'", m.message)
	}

	// Verify DB was updated
	task, err := d.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Status != "done" {
		t.Errorf("task status = %q, want 'done'", task.Status)
	}
}

func TestTasksModel_ChangeStatusCancel(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Fix bug", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", 0, "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Navigate to task line
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	// Press 's' then esc
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.mode != modePickStatus {
		t.Fatalf("mode = %d, want modePickStatus", m.mode)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal after esc", m.mode)
	}
}

func TestTasksModel_ChangeStatusNoTask(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Fix bug", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", 0, "auto")

	m := NewTasksModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Cursor at 0 = project line (no taskID)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal (s on project line should be no-op)", m.mode)
	}
}

func TestTasksModel_DateFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID1, _ := d.InsertTask(1, "Today task", "", "{}")
	d.InsertAction("today-action", &taskID1, "{}", "pending", 0, "auto")

	taskID2, _ := d.InsertTask(1, "Old task", "", "{}")
	d.InsertAction("old-action", &taskID2, "{}", "pending", 0, "auto")

	// Set old-action's created_at to a different date
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE template_id = 'old-action'")

	// Get today's date from the first action
	actions, _ := d.ListActions("", nil)
	var todayDate string
	for _, a := range actions {
		if a.TemplateID == "today-action" {
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

func TestTasksModel_SetSize(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewTasksModel(d, "")
	m = m.SetSize(100, 50)

	if m.width != 100 || m.height != 50 {
		t.Errorf("size = %dx%d, want 100x50", m.width, m.height)
	}
}
