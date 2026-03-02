package view

import (
	"fmt"
	"os"
	"strings"

	"github.com/MH4GF/tq/db"
)

// TaskView represents a task with its actions for rendering.
type TaskView struct {
	Task    db.Task
	Actions []db.Action
}

// ProjectView represents a project with its tasks for rendering.
type ProjectView struct {
	Project db.Project
	Tasks   []TaskView
}

// Generate produces markdown from the DB for all projects with active tasks.
func Generate(database *db.DB) (string, error) {
	projects, err := database.ListProjects()
	if err != nil {
		return "", fmt.Errorf("list projects: %w", err)
	}

	var sections []string
	for _, p := range projects {
		section, err := generateProjectSection(database, p)
		if err != nil {
			return "", err
		}
		if section != "" {
			sections = append(sections, section)
		}
	}

	if len(sections) == 0 {
		return "", nil
	}
	return strings.Join(sections, "\n"), nil
}

func generateProjectSection(database *db.DB, project db.Project) (string, error) {
	tasks, err := database.ListTasksByProject(project.ID)
	if err != nil {
		return "", fmt.Errorf("list tasks for %s: %w", project.Name, err)
	}
	if len(tasks) == 0 {
		return "", nil
	}

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("### %s\n", project.Name))

	statusGroups := []struct {
		label  string
		status string
	}{
		{"Done", "done"},
		{"In Review", "review"},
		{"In Progress", "open"},
		{"Ready", "ready"},
		{"Blocked", "blocked"},
	}

	for _, sg := range statusGroups {
		var matching []db.Task
		for _, t := range tasks {
			if t.Status == sg.status {
				matching = append(matching, t)
			}
		}
		if len(matching) == 0 {
			continue
		}
		buf.WriteString(fmt.Sprintf("#### %s\n", sg.label))
		for _, t := range matching {
			line := fmt.Sprintf("- #%d %s", t.ID, t.Title)
			if t.URL != "" {
				line += fmt.Sprintf(" [link](%s)", t.URL)
			}

			taskID := t.ID
			actions, err := database.ListActions("", &taskID)
			if err != nil {
				return "", fmt.Errorf("list actions for task %d: %w", t.ID, err)
			}

			buf.WriteString(line + "\n")
			for _, a := range actions {
				checkbox := " "
				if a.Status == "done" {
					checkbox = "x"
				}
				actionLine := fmt.Sprintf("  - [%s] %s", checkbox, a.TemplateID)
				if a.Status == "running" {
					actionLine += " (running)"
				} else if a.Status == "failed" {
					actionLine += " (failed)"
				} else if a.Status == "waiting_human" {
					actionLine += " (waiting)"
				}
				if a.CompletedAt.Valid {
					parts := strings.Split(a.CompletedAt.String, " ")
					if len(parts) >= 2 {
						timeParts := strings.Split(parts[1], ":")
						if len(timeParts) >= 2 {
							actionLine += fmt.Sprintf(" (%s:%s)", timeParts[0], timeParts[1])
						}
					}
				}
				buf.WriteString(actionLine + "\n")
			}
		}
	}

	return buf.String(), nil
}

// Inject replaces content between <!-- tq:start --> and <!-- tq:end --> markers
// in the given file with the generated markdown.
func Inject(filePath string, content string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	text := string(data)
	startMarker := "<!-- tq:start -->"
	endMarker := "<!-- tq:end -->"

	startIdx := strings.Index(text, startMarker)
	endIdx := strings.Index(text, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("markers <!-- tq:start --> and <!-- tq:end --> not found in %s", filePath)
	}
	if startIdx >= endIdx {
		return fmt.Errorf("<!-- tq:start --> must come before <!-- tq:end --> in %s", filePath)
	}

	var replacement string
	if content != "" {
		replacement = startMarker + "\n" + content + endMarker
	} else {
		replacement = startMarker + "\n" + endMarker
	}

	// Find the end of the endMarker (include trailing newline if present)
	endOfMarker := endIdx + len(endMarker)
	if endOfMarker < len(text) && text[endOfMarker] == '\n' {
		endOfMarker++
	}

	newText := text[:startIdx] + replacement + "\n" + text[endOfMarker:]

	return os.WriteFile(filePath, []byte(newText), 0644)
}
