package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/testutil"
)

func TestNew(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)
	if m.Screen() != screenTasks {
		t.Errorf("initial screen = %d, want screenTasks(0)", m.Screen())
	}
	if m.IsQuitting() {
		t.Error("should not be quitting initially")
	}
}

func TestNavigateToQueueAndBack(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", "auto")

	m := New(d, nil)
	// Load tasks
	msg := m.tasks.Init()()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	// Navigate cursor to task line (line 0 = project, line 1 = task)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)

	// Press enter to select task → should emit taskSelectedMsg
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)

	if cmd != nil {
		selMsg := cmd()
		updated, _ = m.Update(selMsg)
		m = updated.(Model)
	}

	if m.Screen() != screenQueue {
		t.Errorf("after selecting task, screen = %d, want screenQueue(1)", m.Screen())
	}

	// Press q to go back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.Screen() != screenTasks {
		t.Errorf("after q, screen = %d, want screenTasks(0)", m.Screen())
	}
}

func TestNavigateBackWithEsc(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", "auto")

	m := New(d, nil)
	msg := m.tasks.Init()()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	// Navigate to task and select
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(Model)
	}

	if m.Screen() != screenQueue {
		t.Fatalf("should be on queue screen")
	}

	// Press esc to go back
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.Screen() != screenTasks {
		t.Errorf("after esc, screen = %d, want screenTasks(0)", m.Screen())
	}
}

func TestQuit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)

	if !m.IsQuitting() {
		t.Error("should be quitting after 'q' on tasks screen")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
	if m.View() != "" {
		t.Errorf("quitting view should be empty, got %q", m.View())
	}
}

func TestQuitFromQueueNotQuit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "", "{}")
	d.InsertAction("check", &taskID, "{}", "pending", "auto")

	m := New(d, nil)
	msg := m.tasks.Init()()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	// Navigate to queue
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(Model)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if cmd != nil {
		updated, _ = m.Update(cmd())
		m = updated.(Model)
	}

	// q from queue should go back, not quit
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)
	if m.IsQuitting() {
		t.Error("q from queue screen should go back, not quit")
	}
	if m.Screen() != screenTasks {
		t.Error("should be back on tasks screen")
	}
}

func TestWindowResize(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if m.width != 120 || m.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", m.width, m.height)
	}
}

func TestInit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a batch command")
	}
}

func TestViewContainsHeader(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)
	view := m.View()
	if !contains(view, "Tasks") {
		t.Errorf("view should contain 'Tasks', got %q", view)
	}
}

func TestApp_DateFilterDefault(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)

	if m.queue.dateFilter == "" {
		t.Error("initial queue dateFilter should be today's date, got empty")
	}
	if m.tasks.dateFilter == "" {
		t.Error("initial tasks dateFilter should be today's date, got empty")
	}
}

func TestHelpText(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil)

	// Tasks screen help
	view := m.View()
	if !contains(view, "j/k: navigate") {
		t.Errorf("tasks help missing navigate, got %q", view)
	}
	if !contains(view, "enter: select") {
		t.Errorf("tasks help missing select, got %q", view)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
