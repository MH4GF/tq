package tui

import (
	"errors"
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
	toggleErr error
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
	if s.toggleErr != nil {
		return s.toggleErr
	}
	return s.Store.UpdateScheduleEnabled(id, enabled)
}

func TestSchedulesModel_LoadSchedules_SurfacesError(t *testing.T) {
	d := testutil.NewTestDB(t)
	stub := &schedulesErrorStore{Store: d, listErr: errors.New("list broken")}

	m := NewSchedulesModel(stub)
	loaded, ok := m.loadSchedules()().(schedulesLoadedMsg)
	if !ok {
		t.Fatalf("msg type = %T, want schedulesLoadedMsg", loaded)
	}
	if loaded.err == nil {
		t.Fatal("expected loadSchedules to surface error")
	}

	m, _ = m.Update(loaded)
	if !m.messageIsError {
		t.Errorf("messageIsError = false, want true")
	}
	if !strings.Contains(m.message, "load schedules failed") {
		t.Errorf("m.message = %q, want prefix 'load schedules failed'", m.message)
	}
	if !strings.Contains(m.View(), "load schedules failed") {
		t.Errorf("view should surface error, got %q", m.View())
	}
}

func TestSchedulesModel_KeyOpsSurfaceError(t *testing.T) {
	tests := []struct {
		name        string
		stub        *schedulesErrorStore
		key         rune
		wantSubstrs []string
	}{
		{
			name:        "delete error",
			stub:        &schedulesErrorStore{deleteErr: errors.New("delete broken")},
			key:         'd',
			wantSubstrs: []string{"delete schedule", "failed"},
		},
		{
			name:        "toggle error",
			stub:        &schedulesErrorStore{toggleErr: errors.New("toggle broken")},
			key:         'e',
			wantSubstrs: []string{"toggle schedule", "failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, err := d.InsertTask(1, "sched task", "{}", "")
			if err != nil {
				t.Fatalf("insert task: %v", err)
			}
			if _, err := d.InsertSchedule(taskID, "do thing", "Do Thing", "0 * * * *", "{}"); err != nil {
				t.Fatalf("insert schedule: %v", err)
			}

			tt.stub.Store = d
			m := NewSchedulesModel(tt.stub)
			m, _ = m.Update(m.Init()())

			if m.selectedSchedule() == nil {
				t.Fatalf("expected selected schedule, got nil")
			}

			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}})

			if !m.messageIsError {
				t.Errorf("messageIsError = false, want true")
			}
			for _, want := range tt.wantSubstrs {
				if !strings.Contains(m.message, want) {
					t.Errorf("m.message = %q, want substring %q", m.message, want)
				}
			}
		})
	}
}
