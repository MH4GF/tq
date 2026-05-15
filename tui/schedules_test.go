package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MH4GF/tq/db"
)

type schedulesErrorStore struct {
	db.Store
	listErr   error
	deleteErr error
	updateErr error
}

func (s *schedulesErrorStore) ListSchedules(limit int) ([]db.Schedule, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Store.ListSchedules(limit)
}

func (s *schedulesErrorStore) DeleteSchedule(id int64) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return s.Store.DeleteSchedule(id)
}

func (s *schedulesErrorStore) UpdateScheduleEnabled(id int64, enabled bool) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.Store.UpdateScheduleEnabled(id, enabled)
}

// A failed ListSchedules must surface a non-empty error message instead of
// rendering as a silent empty "No schedules" list.
func TestSchedulesModel_ListSchedulesError(t *testing.T) {
	store := &schedulesErrorStore{listErr: errors.New("database is locked")}
	m := NewSchedulesModel(store)

	msg := m.loadSchedules()()
	loaded, ok := msg.(schedulesLoadedMsg)
	if !ok {
		t.Fatalf("expected schedulesLoadedMsg, got %T", msg)
	}
	if loaded.err == nil {
		t.Fatal("expected schedulesLoadedMsg.err to be set on ListSchedules failure")
	}

	m, _ = m.Update(loaded)

	if !m.messageIsError {
		t.Error("expected messageIsError to be true after failed load")
	}
	if m.message == "" {
		t.Error("expected a non-empty error message after failed load")
	}
	if !strings.Contains(m.message, "database is locked") {
		t.Errorf("expected message to contain the underlying error, got %q", m.message)
	}

	view := m.View()
	if strings.Contains(view, "No schedules") {
		t.Errorf("failed load must not render as silent empty list, got: %q", view)
	}
	if !strings.Contains(view, "database is locked") {
		t.Errorf("expected View to surface the error, got: %q", view)
	}
}

// A failed UpdateScheduleEnabled must surface a visible error message instead
// of leaving the user with no feedback after pressing 'e'.
func TestSchedulesModel_ToggleEnabledError(t *testing.T) {
	store := &schedulesErrorStore{updateErr: errors.New("database is locked")}
	m := NewSchedulesModel(store)
	m.schedules = []db.Schedule{{ID: 7, Enabled: true}}

	m, _ = m.toggleEnabled(&m.schedules[0])

	if !m.messageIsError {
		t.Error("expected messageIsError to be true after failed toggle")
	}
	if !strings.Contains(m.message, "database is locked") {
		t.Errorf("expected message to contain the underlying error, got %q", m.message)
	}
	if !strings.Contains(m.message, "#7") {
		t.Errorf("expected message to reference schedule #7, got %q", m.message)
	}
}

// A failed DeleteSchedule must surface a visible error message instead of the
// reload silently re-rendering the unchanged schedule with no feedback.
func TestSchedulesModel_DeleteScheduleError(t *testing.T) {
	store := &schedulesErrorStore{deleteErr: errors.New("foreign key constraint")}
	m := NewSchedulesModel(store)
	m.schedules = []db.Schedule{{ID: 3, Enabled: true}}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})

	if !m.messageIsError {
		t.Error("expected messageIsError to be true after failed delete")
	}
	if !strings.Contains(m.message, "foreign key constraint") {
		t.Errorf("expected message to contain the underlying error, got %q", m.message)
	}
	if !strings.Contains(m.message, "#3") {
		t.Errorf("expected message to reference schedule #3, got %q", m.message)
	}
}
