package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/testutil"
)

func TestQueueModel_Empty(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewQueueModel(d, "")
	view := m.View()
	if !contains(view, "No actions") {
		t.Errorf("empty view should show 'No actions', got %q", view)
	}
}

func TestQueueModel_LoadActions(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "", "{}")
	d.InsertAction("check-pr", &taskID, "{}", "pending", "auto")
	d.InsertAction("fix-ci", &taskID, "{}", "running", "auto")

	m := NewQueueModel(d, "")

	// Simulate load
	cmd := m.Init()
	msg := cmd()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "check-pr") {
		t.Errorf("view should contain 'check-pr', got %q", view)
	}
	if !contains(view, "fix-ci") {
		t.Errorf("view should contain 'fix-ci', got %q", view)
	}
}

func TestQueueModel_Navigation(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("a", nil, "{}", "pending", "auto")
	d.InsertAction("b", nil, "{}", "pending", "auto")
	d.InsertAction("c", nil, "{}", "pending", "auto")

	m := NewQueueModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}

	// Move down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 1 {
		t.Errorf("after j, cursor = %d, want 1", m.cursor)
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("after 2nd j, cursor = %d, want 2", m.cursor)
	}

	// Can't go past end
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.cursor != 2 {
		t.Errorf("at end, cursor = %d, want 2", m.cursor)
	}

	// Move up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 1 {
		t.Errorf("after k, cursor = %d, want 1", m.cursor)
	}

	// Can't go before start
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.cursor != 0 {
		t.Errorf("at start, cursor = %d, want 0", m.cursor)
	}
}

func TestQueueModel_Reload(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewQueueModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	if len(m.actions) != 0 {
		t.Errorf("initial actions = %d, want 0", len(m.actions))
	}

	// Insert after initial load
	d.InsertAction("new-one", nil, "{}", "pending", "auto")

	// Reload
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		reloadMsg := cmd()
		m, _ = m.Update(reloadMsg)
	}

	if len(m.actions) != 1 {
		t.Errorf("after reload, actions = %d, want 1", len(m.actions))
	}
}

func TestQueueModel_StatusIcons(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("pending-action", nil, "{}", "pending", "auto")
	d.InsertAction("running-action", nil, "{}", "running", "auto")

	m := NewQueueModel(d, "")
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "○") {
		t.Errorf("view should contain pending icon ○, got %q", view)
	}
	if !contains(view, "●") {
		t.Errorf("view should contain running icon ●, got %q", view)
	}
}

func TestQueueModel_DateFilter(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "", "{}")
	d.InsertAction("today-action", &taskID, "{}", "pending", "auto")
	d.InsertAction("old-action", &taskID, "{}", "pending", "auto")

	// Set old-action's created_at to a different date
	d.Exec("UPDATE actions SET created_at = '2025-01-01 00:00:00' WHERE prompt_id = 'old-action'")

	// Get today's date from the first action
	actions, _ := d.ListActions("", nil)
	var todayDate string
	for _, a := range actions {
		if a.PromptID == "today-action" {
			todayDate = a.CreatedAt[:10]
			break
		}
	}

	m := NewQueueModel(d, todayDate)
	msg := m.Init()()
	m, _ = m.Update(msg)

	if len(m.actions) != 1 {
		t.Errorf("filtered actions = %d, want 1", len(m.actions))
	}

	view := m.View()
	if !contains(view, "today-action") {
		t.Errorf("view should contain 'today-action', got %q", view)
	}
	if contains(view, "old-action") {
		t.Errorf("view should not contain 'old-action', got %q", view)
	}

	// Without filter, both should appear
	m2 := NewQueueModel(d, "")
	msg2 := m2.Init()()
	m2, _ = m2.Update(msg2)

	if len(m2.actions) != 2 {
		t.Errorf("unfiltered actions = %d, want 2", len(m2.actions))
	}
}

func TestQueueModel_InlineResult(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	taskID, _ := d.InsertTask(1, "Test task", "", "{}")
	id, _ := d.InsertAction("check-pr", &taskID, "{}", "running", "auto")
	d.MarkDone(id, "all checks passed")

	m := NewQueueModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "result: all checks passed") {
		t.Errorf("view should contain inline result, got %q", view)
	}
}

func TestQueueModel_InlineResultFailed(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("deploy", nil, "{}", "running", "auto")
	d.MarkFailed(id, "timeout error")

	m := NewQueueModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	view := m.View()
	if !contains(view, "result: timeout error") {
		t.Errorf("view should contain inline result for failed action, got %q", view)
	}
}

func TestQueueModel_DetailView(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("check", nil, "{}", "running", "auto")
	d.MarkDone(id, "detailed result\nline 2\nline 3")

	m := NewQueueModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	if m.InDetailView() {
		t.Error("should not be in detail view initially")
	}

	// Press v to enter detail view
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.InDetailView() {
		t.Error("should be in detail view after pressing v")
	}

	view := m.View()
	if !contains(view, "Action Detail") {
		t.Errorf("detail view should contain header, got %q", view)
	}
	if !contains(view, "detailed result") {
		t.Errorf("detail view should contain result text, got %q", view)
	}

	// Press q to exit
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.InDetailView() {
		t.Error("should exit detail view after pressing q")
	}
}

func TestQueueModel_DetailViewNoResult(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	d.InsertAction("check", nil, "{}", "pending", "auto")

	m := NewQueueModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Press v on action with no result - should be no-op
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.InDetailView() {
		t.Error("v should be no-op when action has no result")
	}
}

func TestQueueModel_DetailViewScroll(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("check", nil, "{}", "running", "auto")
	d.MarkDone(id, "line1\nline2\nline3")

	m := NewQueueModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if m.detailScroll != 0 {
		t.Errorf("initial scroll = %d, want 0", m.detailScroll)
	}

	// Scroll down
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.detailScroll != 1 {
		t.Errorf("after j, scroll = %d, want 1", m.detailScroll)
	}

	// Scroll up
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.detailScroll != 0 {
		t.Errorf("after k, scroll = %d, want 0", m.detailScroll)
	}

	// Can't go below 0
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.detailScroll != 0 {
		t.Errorf("scroll should not go below 0, got %d", m.detailScroll)
	}
}

func TestQueueModel_DetailViewEscIgnored(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	id, _ := d.InsertAction("check", nil, "{}", "running", "auto")
	d.MarkDone(id, "some result")

	m := NewQueueModel(d, "")
	m = m.SetSize(120, 40)
	msg := m.Init()()
	m, _ = m.Update(msg)

	// Enter detail view
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if !m.InDetailView() {
		t.Fatal("should be in detail view")
	}

	// Press esc — should be ignored
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !m.InDetailView() {
		t.Error("esc should be ignored in detail view")
	}
}

func TestQueueModel_SetSize(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)

	m := NewQueueModel(d, "")
	m = m.SetSize(100, 50)

	if m.width != 100 || m.height != 50 {
		t.Errorf("size = %dx%d, want 100x50", m.width, m.height)
	}
}
