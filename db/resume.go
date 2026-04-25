package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const DefaultResumeMessage = "Continue the previous session."

// ResumeOptions controls how a resume action is created.
type ResumeOptions struct {
	Message string // additional instruction for the resumed session (default: DefaultResumeMessage)
	Mode    string // execution mode (default: parent action's mode)
}

// ResumeAction creates a new pending action that resumes the parent action's
// claude session via `claude --resume <session_id>`. Only the session id is
// inherited; other claude_args (--worktree, --permission-mode, etc.) are
// dropped because the resumed claude session restores its own context.
func (db *DB) ResumeAction(parentID int64, opts ResumeOptions) (int64, error) {
	parent, err := db.GetAction(parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("parent action #%d not found", parentID)
		}
		return 0, fmt.Errorf("get parent action #%d: %w", parentID, err)
	}

	if !IsTerminalActionStatus(parent.Status) {
		return 0, fmt.Errorf("only failed/cancelled/done actions can be resumed (got: %s)", parent.Status)
	}

	parentMeta := make(map[string]any)
	if parent.Metadata != "" && parent.Metadata != "{}" {
		if err := json.Unmarshal([]byte(parent.Metadata), &parentMeta); err != nil {
			return 0, fmt.Errorf("parse parent metadata: %w", err)
		}
	}

	sessionID, _ := parentMeta["claude_session_id"].(string)
	if strings.TrimSpace(sessionID) == "" {
		return 0, fmt.Errorf("action #%d has no claude_session_id; cannot resume", parentID)
	}

	mode := opts.Mode
	if mode == "" {
		if m, ok := parentMeta["mode"].(string); ok && m != "" {
			mode = m
		} else {
			mode = "interactive"
		}
	}

	message := opts.Message
	if strings.TrimSpace(message) == "" {
		message = DefaultResumeMessage
	}

	meta := map[string]any{
		"instruction":       message,
		"mode":              mode,
		"claude_args":       []string{"--resume", sessionID},
		"claude_session_id": sessionID,
		"parent_action_id":  parentID,
		"is_resume":         true,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return 0, fmt.Errorf("marshal metadata: %w", err)
	}

	title := fmt.Sprintf("resume #%d", parentID)
	newID, err := db.InsertAction(title, parent.TaskID, string(metaJSON), ActionStatusPending, nil)
	if err != nil {
		return 0, fmt.Errorf("insert resume action: %w", err)
	}
	return newID, nil
}
