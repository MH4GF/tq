package tui

import (
	"errors"
	"fmt"
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
