package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
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

// Failed store operations must surface a visible error message instead of
// leaving the user with no feedback (silent empty list on load, or a reload
// that silently re-renders the unchanged schedule on toggle/delete).
func TestSchedulesModel_ErrorSurfacing(t *testing.T) {
	tests := []struct {
		name          string
		storeErr      func(*schedulesErrorStore)
		invoke        func(*SchedulesModel) SchedulesModel
		schedule      db.Schedule
		wantErrSubstr string
		extraCheck    func(t *testing.T, m SchedulesModel)
	}{
		{
			name:     "ListSchedules error",
			storeErr: func(s *schedulesErrorStore) { s.listErr = errors.New("database is locked") },
			invoke: func(m *SchedulesModel) SchedulesModel {
				next, _ := m.Update(m.loadSchedules()())
				return next
			},
			wantErrSubstr: "database is locked",
			extraCheck: func(t *testing.T, m SchedulesModel) {
				t.Helper()
				view := m.View()
				if strings.Contains(view, "No schedules") {
					t.Errorf("failed load must not render as silent empty list, got: %q", view)
				}
				if !strings.Contains(view, "database is locked") {
					t.Errorf("expected View to surface the error, got: %q", view)
				}
			},
		},
		{
			name:     "ToggleEnabled error",
			storeErr: func(s *schedulesErrorStore) { s.updateErr = errors.New("database is locked") },
			invoke: func(m *SchedulesModel) SchedulesModel {
				next, _ := m.toggleEnabled(&m.schedules[0])
				return next
			},
			schedule:      db.Schedule{ID: 7, Enabled: true},
			wantErrSubstr: "database is locked",
		},
		{
			name:     "DeleteSchedule error",
			storeErr: func(s *schedulesErrorStore) { s.deleteErr = errors.New("foreign key constraint") },
			invoke: func(m *SchedulesModel) SchedulesModel {
				next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
				return next
			},
			schedule:      db.Schedule{ID: 3, Enabled: true},
			wantErrSubstr: "foreign key constraint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &schedulesErrorStore{}
			tc.storeErr(store)
			m := NewSchedulesModel(store)
			if tc.schedule.ID != 0 {
				m.schedules = []db.Schedule{tc.schedule}
			}

			m = tc.invoke(&m)

			if !m.messageIsError {
				t.Error("expected messageIsError to be true")
			}
			if !strings.Contains(m.message, tc.wantErrSubstr) {
				t.Errorf("expected message to contain %q, got %q", tc.wantErrSubstr, m.message)
			}
			if tc.schedule.ID != 0 {
				idMarker := fmt.Sprintf("#%d", tc.schedule.ID)
				if !strings.Contains(m.message, idMarker) {
					t.Errorf("expected message to reference schedule %s, got %q", idMarker, m.message)
				}
			}
			if tc.extraCheck != nil {
				tc.extraCheck(t, m)
			}
		})
	}
}

func TestSchedulesModel_MessageAutoClearsAfterTTL(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test", "{}", "")
	schedID, _ := d.InsertSchedule(taskID, "instr", "Title", "* * * * *", "{}")

	m := NewSchedulesModel(d)
	m.schedules = []db.Schedule{{ID: schedID, TaskID: taskID, Enabled: true}}

	updated, cmd := m.toggleEnabled(&m.schedules[0])
	m = updated
	if m.message == "" {
		t.Fatal("message should be set after toggleEnabled")
	}
	if cmd == nil {
		t.Fatal("expected a batched cmd after toggleEnabled")
	}

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.message == "" {
		t.Error("keystroke should not clear message; TTL should")
	}

	m, _ = m.Update(clearSchedulesMessageMsg{gen: m.messageGen})
	if m.message != "" {
		t.Errorf("message should be cleared after clearSchedulesMessageMsg, got %q", m.message)
	}
	if m.messageIsError {
		t.Error("messageIsError should be reset alongside message")
	}
}

func TestSchedulesModel_MessageStaleTimerIgnored(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "test", "{}", "")
	id1, _ := d.InsertSchedule(taskID, "a", "A", "* * * * *", "{}")
	id2, _ := d.InsertSchedule(taskID, "b", "B", "* * * * *", "{}")

	m := NewSchedulesModel(d)
	m.schedules = []db.Schedule{
		{ID: id1, TaskID: taskID, Enabled: true},
		{ID: id2, TaskID: taskID, Enabled: true},
	}

	updated, _ := m.toggleEnabled(&m.schedules[0])
	m = updated
	staleGen := m.messageGen

	updated, _ = m.toggleEnabled(&m.schedules[1])
	m = updated
	wantSecond := m.message
	if wantSecond == "" {
		t.Fatal("second toggle should produce a message")
	}

	m, _ = m.Update(clearSchedulesMessageMsg{gen: staleGen})
	if m.message != wantSecond {
		t.Errorf("stale clear should not wipe newer message, got %q want %q", m.message, wantSecond)
	}

	m, _ = m.Update(clearSchedulesMessageMsg{gen: m.messageGen})
	if m.message != "" {
		t.Errorf("current-gen clear should wipe message, got %q", m.message)
	}
}
