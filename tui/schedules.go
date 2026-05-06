package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MH4GF/tq/db"
)

type schedulesMode int

const (
	schedModeNormal schedulesMode = iota
	schedModeDetail
)

type SchedulesModel struct {
	schedules      []db.Schedule
	cursor         int
	width          int
	height         int
	database       db.Store
	message        string
	messageIsError bool
	mode           schedulesMode
	detailIdx      int
}

type schedulesLoadedMsg struct {
	schedules []db.Schedule
	err       error
}

func NewSchedulesModel(database db.Store) SchedulesModel {
	return SchedulesModel{database: database}
}

func (m SchedulesModel) loadSchedules() tea.Cmd {
	return func() tea.Msg {
		schedules, err := m.database.ListSchedules(0)
		if err != nil {
			return schedulesLoadedMsg{err: err}
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
		if msg.err != nil {
			m.message = fmt.Sprintf("load schedules failed: %v", msg.err)
			m.messageIsError = true
		}
		m.schedules = msg.schedules
		if m.cursor >= len(m.schedules) {
			m.cursor = max(0, len(m.schedules)-1)
		}
	case tea.KeyMsg:
		if m.mode == schedModeDetail {
			return m.updateDetail(msg)
		}
		m.message = ""
		m.messageIsError = false
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("j", "down"))):
			if m.cursor < len(m.schedules)-1 {
				m.cursor++
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("k", "up"))):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("v", "enter"))):
			if m.selectedSchedule() != nil {
				m.detailIdx = m.cursor
				m.mode = schedModeDetail
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
			if s := m.selectedSchedule(); s != nil {
				return m.toggleEnabled(s)
			}
		case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
			if s := m.selectedSchedule(); s != nil {
				if err := m.database.DeleteSchedule(s.ID); err != nil {
					m.message = fmt.Sprintf("delete schedule #%d failed: %v", s.ID, err)
					m.messageIsError = true
				} else {
					m.message = fmt.Sprintf("schedule #%d deleted", s.ID)
				}
				return m, m.loadSchedules()
			}
		}
	}
	return m, nil
}

func (m SchedulesModel) updateDetail(msg tea.KeyMsg) (SchedulesModel, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "esc"))):
		m.mode = schedModeNormal
	case key.Matches(msg, key.NewBinding(key.WithKeys("e"))):
		if m.detailIdx >= 0 && m.detailIdx < len(m.schedules) {
			return m.toggleEnabled(&m.schedules[m.detailIdx])
		}
	}
	return m, nil
}

func (m SchedulesModel) toggleEnabled(s *db.Schedule) (SchedulesModel, tea.Cmd) {
	newEnabled := !s.Enabled
	if err := m.database.UpdateScheduleEnabled(s.ID, newEnabled); err != nil {
		m.message = fmt.Sprintf("toggle schedule #%d failed: %v", s.ID, err)
		m.messageIsError = true
	} else {
		action := "enabled"
		if !newEnabled {
			action = "disabled"
		}
		m.message = fmt.Sprintf("schedule #%d %s", s.ID, action)
		m.messageIsError = false
	}
	return m, m.loadSchedules()
}

func (m SchedulesModel) selectedSchedule() *db.Schedule {
	if m.cursor >= 0 && m.cursor < len(m.schedules) {
		return &m.schedules[m.cursor]
	}
	return nil
}

func (m SchedulesModel) View() string {
	if m.mode == schedModeDetail && m.detailIdx >= 0 && m.detailIdx < len(m.schedules) {
		return m.renderDetail(&m.schedules[m.detailIdx])
	}

	if len(m.schedules) == 0 {
		body := styleMuted.Render("  No schedules")
		if msg := m.renderMessage(); msg != "" {
			body += "\n\n  " + msg
		}
		return body
	}

	const (
		titleW   = 30
		nextRunW = 18
	)

	var b strings.Builder

	// Header
	header := "  " + padRight("TITLE", titleW) + " " + padRight("NEXT RUN", nextRunW) + " " + "LAST RUN"
	b.WriteString(styleTableHeader.Render(header) + "\n")
	b.WriteString(styleBorderChar.Render(strings.Repeat("─", min(m.width, 100))) + "\n")

	visible := m.visibleRange()
	for i := visible.start; i < visible.end; i++ {
		s := m.schedules[i]

		// State: dot only
		dot := styleDone.Render("●")
		if !s.Enabled {
			dot = styleMuted.Render("○")
		}

		nextRun := "—"
		if s.Enabled {
			nextRun = m.computeNextRun(s)
		}

		lastRun := "never"
		if s.LastRunAt.Valid {
			lastRun = db.FormatLocal(s.LastRunAt.String)[:16]
		}

		// Truncate title
		title := s.Title
		if lipgloss.Width(title) > titleW {
			title = truncateDisplay(title, titleW-1) + "…"
		}

		line := " " + dot + " " + padRight(title, titleW) + " " +
			padRight(styleFieldValue.Render(nextRun), nextRunW) + " " +
			styleMuted.Render(lastRun)

		if i == m.cursor {
			line = highlightLine(line, m.width)
		}

		b.WriteString(line + "\n")
	}

	if msg := m.renderMessage(); msg != "" {
		b.WriteString("\n  " + msg + "\n")
	}

	return b.String()
}

func (m SchedulesModel) renderMessage() string {
	if m.message == "" {
		return ""
	}
	style := styleDone
	if m.messageIsError {
		style = styleWarning
	}
	return style.Render(m.message)
}

func (m SchedulesModel) renderDetail(s *db.Schedule) string {
	var b strings.Builder
	pad := "  "
	bodyW := max(0, min(m.width, 80)-len(pad))

	b.WriteString("\n")

	// Header: ← esc  title  enabled/disabled
	dot := styleDone.Render("●")
	stateLabel := styleDone.Render("enabled")
	if !s.Enabled {
		dot = styleMuted.Render("○")
		stateLabel = styleMuted.Render("disabled")
	}
	headerLine := fmt.Sprintf("%s%s  %s  %s %s",
		pad,
		styleDetailBack.Render("← esc"),
		lipgloss.NewStyle().Bold(true).Render(s.Title),
		dot,
		stateLabel,
	)
	b.WriteString(headerLine + "\n")
	b.WriteString(pad + styleBorderChar.Render(strings.Repeat("─", bodyW)) + "\n")

	// Metadata
	nextRun := "—"
	if s.Enabled {
		nextRun = m.computeNextRun(*s)
	}
	lastRun := "never"
	if s.LastRunAt.Valid {
		lastRun = db.FormatLocal(s.LastRunAt.String)[:16]
	}

	fields := []struct{ label, value string }{
		{"       ID", fmt.Sprintf("#%d", s.ID)},
		{"     Task", fmt.Sprintf("#%d", s.TaskID)},
		{"     Cron", s.CronExpr},
		{" Next Run", nextRun},
		{" Last Run", lastRun},
		{"  Created", db.FormatLocal(s.CreatedAt)[:16]},
	}
	for _, f := range fields {
		fmt.Fprintf(&b, "%s%s  %s\n",
			pad,
			styleFieldLabel.Render(f.label),
			styleFieldValue.Render(f.value),
		)
	}
	b.WriteString(pad + styleBorderChar.Render(strings.Repeat("─", bodyW)) + "\n")

	// Instruction
	b.WriteString("\n")
	b.WriteString(pad + styleMuted.Render("Instruction:") + "\n")
	for raw := range strings.SplitSeq(s.Instruction, "\n") {
		for _, line := range wrapLine(raw, bodyW) {
			b.WriteString(pad + line + "\n")
		}
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
	if m.mode == schedModeDetail {
		return []HelpKey{
			{"esc/q", "back"},
			{"e", "enable/disable"},
		}
	}
	keys := commonHelpKeys()
	if m.selectedSchedule() != nil {
		keys = append(keys, HelpKey{"v/enter", "view detail"}, HelpKey{"e", "enable/disable"}, HelpKey{"d", "delete"})
	}
	return keys
}

func (m SchedulesModel) SetSize(w, h int) SchedulesModel {
	m.width = w
	m.height = h
	return m
}

func (m SchedulesModel) Mode() schedulesMode {
	return m.mode
}
