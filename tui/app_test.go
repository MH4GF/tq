package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MH4GF/tq/testutil"
)

func TestNew(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)
	if m.ActiveTab() != tabTasks {
		t.Errorf("initial tab = %d, want tabTasks(0)", m.ActiveTab())
	}
	if m.IsQuitting() {
		t.Error("should not be quitting initially")
	}
}

func TestTabSwitch(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)

	// Tab key switches Tasks → Schedules
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.ActiveTab() != tabSchedules {
		t.Errorf("after tab, active = %d, want tabSchedules(1)", m.ActiveTab())
	}

	// Schedules → Tasks
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.ActiveTab() != tabTasks {
		t.Errorf("after 2nd tab, active = %d, want tabTasks(0)", m.ActiveTab())
	}

	// Number keys
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(Model)
	if m.ActiveTab() != tabSchedules {
		t.Errorf("after '2', active = %d, want tabSchedules(1)", m.ActiveTab())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = updated.(Model)
	if m.ActiveTab() != tabTasks {
		t.Errorf("after '1', active = %d, want tabTasks(0)", m.ActiveTab())
	}
}

func TestQuit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(Model)

	if !m.IsQuitting() {
		t.Error("should be quitting after 'q'")
	}
	if cmd == nil {
		t.Error("expected tea.Quit cmd")
	}
	if m.View() != "" {
		t.Errorf("quitting view should be empty, got %q", m.View())
	}
}

func TestWindowResize(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if m.width != 120 || m.height != 40 {
		t.Errorf("size = %dx%d, want 120x40", m.width, m.height)
	}
}

func TestInit(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return a batch command")
	}
}

func TestViewContainsTabs(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)
	view := m.View()
	if !strings.Contains(view, "Tasks") {
		t.Errorf("view should contain 'Tasks', got %q", view)
	}
	if !strings.Contains(view, "Schedules") {
		t.Errorf("view should contain 'Schedules', got %q", view)
	}
}

func TestApp_DateFilterDefault(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)

	if m.tasks.dateFilter == "" {
		t.Error("initial tasks dateFilter should be today's date, got empty")
	}
}

func TestHelpText(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)

	// Tasks tab help (default tab)
	view := m.View()
	if !strings.Contains(view, "j/k") || !strings.Contains(view, "navigate") {
		t.Errorf("tasks help missing navigate, got %q", view)
	}
	if !strings.Contains(view, "tab") || !strings.Contains(view, "switch") {
		t.Errorf("tasks help missing tab switch, got %q", view)
	}
}
