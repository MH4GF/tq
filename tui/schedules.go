package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MH4GF/tq/db"
)

type SchedulesModel struct {
	schedules []db.Schedule
	cursor    int
	width     int
	height    int
	database  db.Store
	message   string
}

type schedulesLoadedMsg struct {
	schedules []db.Schedule
}

func NewSchedulesModel(database db.Store) SchedulesModel {
	return SchedulesModel{database: database}
}

func (m SchedulesModel) loadSchedules() tea.Cmd {
	return func() tea.Msg {
		schedules, err := m.database.ListSchedules()
		if err != nil {
			return schedulesLoadedMsg{}
		}
		return schedulesLoadedMsg{schedules: schedules}
	}
}

func (m SchedulesModel) Init() tea.Cmd {
	return m.loadSchedules()
}

func (m SchedulesModel) Update(msg tea.Msg) (SchedulesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case schedulesLoadedMsg:
		m.schedules = msg.schedules
		if m.cursor >= len(m.schedules) {
			m.cursor = max(0, len(m.schedules)-1)
		}
	case tea.KeyMsg:
		m.message = ""
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.schedules)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			if s := m.selectedSchedule(); s != nil {
				newEnabled := !s.Enabled
				if err := m.database.UpdateScheduleEnabled(s.ID, newEnabled); err == nil {
					action := "enabled"
					if !newEnabled {
						action = "disabled"
					}
					m.message = fmt.Sprintf("schedule #%d %s", s.ID, action)
				}
				return m, m.loadSchedules()
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			if s := m.selectedSchedule(); s != nil {
				if err := m.database.DeleteSchedule(s.ID); err == nil {
					m.message = fmt.Sprintf("schedule #%d deleted", s.ID)
				}
				return m, m.loadSchedules()
			}
		}
	}
	return m, nil
}

func (m SchedulesModel) selectedSchedule() *db.Schedule {
	if m.cursor >= 0 && m.cursor < len(m.schedules) {
		return &m.schedules[m.cursor]
	}
	return nil
}

func (m SchedulesModel) View() string {
	if len(m.schedules) == 0 {
		return styleMuted.Render("  No schedules")
	}

	var b strings.Builder
	header := fmt.Sprintf("  %-4s %-8s %-20s %-16s %-20s %s", "ID", "Enabled", "Title", "Cron", "Next Run", "Last Run")
	b.WriteString(styleMuted.Render(header) + "\n")
	b.WriteString(styleMuted.Render(strings.Repeat("─", min(m.width, 100))) + "\n")

	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		s := m.schedules[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "> "
		}

		enabled := styleDone.Render("yes")
		if !s.Enabled {
			enabled = styleMuted.Render("no ")
		}

		nextRun := "-"
		if s.Enabled {
			nextRun = m.computeNextRun(s)
		}

		lastRun := "-"
		if s.LastRunAt.Valid {
			lastRun = db.FormatLocal(s.LastRunAt.String)[:16]
		}

		line := fmt.Sprintf("%s%-4d %s  %-20s %-16s %-20s %s",
			prefix, s.ID, enabled, s.Title, s.CronExpr, nextRun, lastRun)

		if i == m.cursor {
			line = styleTitle.Render(line)
		}

		b.WriteString(line + "\n")
	}

	if m.message != "" {
		b.WriteString("\n  " + styleDone.Render(m.message) + "\n")
	}

	return b.String()
}

func (m SchedulesModel) computeNextRun(s db.Schedule) string {
	sched, err := db.CronParser.Parse(s.CronExpr)
	if err != nil {
		return "invalid"
	}
	baseTime := s.CreatedAt
	if s.LastRunAt.Valid {
		baseTime = s.LastRunAt.String
	}
	base, err := time.Parse(db.TimeLayout, baseTime)
	if err != nil {
		return "?"
	}
	next := sched.Next(base)
	return next.Local().Format("2006-01-02 15:04")
}

func (m SchedulesModel) visibleRange() visibleRange {
	return calcVisibleRange(m.cursor, len(m.schedules), m.height, 4)
}

func (m SchedulesModel) HelpKeys() []HelpKey {
	keys := commonHelpKeys()
	if m.selectedSchedule() != nil {
		keys = append(keys, HelpKey{"e", "enable/disable"}, HelpKey{"d", "delete"})
	}
	return keys
}

func (m SchedulesModel) SetSize(w, h int) SchedulesModel {
	m.width = w
	m.height = h
	return m
}
