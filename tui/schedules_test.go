package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
)

type schedulesErrorStore struct {
	db.Store
	listErr error
}

func (s *schedulesErrorStore) ListSchedules(limit int) ([]db.Schedule, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Store.ListSchedules(limit)
}

type toggleErrorStore struct {
	db.Store
	toggleErr error
}

func (s *toggleErrorStore) UpdateScheduleEnabled(id int64, enabled bool) error {
	if s.toggleErr != nil {
		return s.toggleErr
	}
	return s.Store.UpdateScheduleEnabled(id, enabled)
}

// A failed UpdateScheduleEnabled must surface an error message instead of
// silently leaving the previous message and reloading as if nothing happened.
func TestSchedulesModel_ToggleEnabledError(t *testing.T) {
	store := &toggleErrorStore{toggleErr: errors.New("database is locked")}
	m := NewSchedulesModel(store)

	s := &db.Schedule{ID: 42, Enabled: true}
	m, _ = m.toggleEnabled(s)

	if !m.messageIsError {
		t.Error("expected messageIsError to be true after failed toggle")
	}
	if !strings.Contains(m.message, "database is locked") {
		t.Errorf("expected message to contain the underlying error, got %q", m.message)
	}
	if !strings.Contains(m.message, "42") {
		t.Errorf("expected message to reference the schedule ID, got %q", m.message)
	}
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
