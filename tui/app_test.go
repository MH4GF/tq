package tui

import (
	"errors"
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

	steps := []struct {
		name    string
		key     tea.KeyMsg
		wantTab tab
	}{
		{"Tab: Tasks→Schedules", tea.KeyMsg{Type: tea.KeyTab}, tabSchedules},
		{"Tab: Schedules→Tasks", tea.KeyMsg{Type: tea.KeyTab}, tabTasks},
		{"Key '2': →Schedules", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}}, tabSchedules},
		{"Key '1': →Tasks", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}}, tabTasks},
	}
	for _, s := range steps {
		updated, _ := m.Update(s.key)
		m = updated.(Model)
		if m.ActiveTab() != s.wantTab {
			t.Errorf("%s: ActiveTab() = %d, want %d", s.name, m.ActiveTab(), s.wantTab)
		}
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

func TestBackgroundStatusError_TTLClear(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)

	updated, cmd := m.Update(backgroundStatusMsg{err: errors.New("boom")})
	m = updated.(Model)

	if !strings.Contains(m.statusLine, "boom") {
		t.Errorf("statusLine = %q, want it to contain error message", m.statusLine)
	}
	if cmd == nil {
		t.Fatal("expected a clear-timer cmd after background error")
	}

	updated, _ = m.Update(clearStatusLineMsg{gen: m.statusLineGen})
	m = updated.(Model)
	if m.statusLine != "" {
		t.Errorf("statusLine should be cleared after clearStatusLineMsg, got %q", m.statusLine)
	}
}

// A stale clear timer (from an earlier error) must not wipe a newer error.
func TestBackgroundStatusError_StaleTimerIgnored(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := New(d, nil, 3)

	updated, _ := m.Update(backgroundStatusMsg{err: errors.New("first")})
	m = updated.(Model)
	staleGen := m.statusLineGen

	updated, _ = m.Update(backgroundStatusMsg{err: errors.New("second")})
	m = updated.(Model)
	if !strings.Contains(m.statusLine, "second") {
		t.Fatalf("statusLine = %q, want second error", m.statusLine)
	}

	updated, _ = m.Update(clearStatusLineMsg{gen: staleGen})
	m = updated.(Model)
	if !strings.Contains(m.statusLine, "second") {
		t.Errorf("stale clear should not wipe newer message, got %q", m.statusLine)
	}

	updated, _ = m.Update(clearStatusLineMsg{gen: m.statusLineGen})
	m = updated.(Model)
	if m.statusLine != "" {
		t.Errorf("current-gen clear should wipe message, got %q", m.statusLine)
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
