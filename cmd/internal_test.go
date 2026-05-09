package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

func TestRunClaudeSessionRecord(t *testing.T) {
	const validPayload = `{"session_id":"abc-123","hook_event_name":"SessionStart","source":"startup"}`

	tests := []struct {
		name          string
		envActionID   string
		seedStatus    string
		seedMetadata  string
		stdin         string
		wantStderrSub string
		wantSessionID string
		wantUnchanged bool
		wantNoMerge   bool // also assert no action.metadata_merged event was emitted
	}{
		{
			name:          "env unset → silent no-op",
			envActionID:   "",
			seedStatus:    db.ActionStatusPending,
			stdin:         validPayload,
			wantUnchanged: true,
		},
		{
			name:          "env not numeric → warn + no-op",
			envActionID:   "not-a-number",
			seedStatus:    db.ActionStatusPending,
			stdin:         validPayload,
			wantStderrSub: "invalid TQ_ACTION_ID",
			wantUnchanged: true,
		},
		{
			name:          "stdin not JSON → warn + no-op",
			envActionID:   "1",
			seedStatus:    db.ActionStatusPending,
			stdin:         "not json",
			wantStderrSub: "parse payload",
			wantUnchanged: true,
		},
		{
			name:          "payload missing session_id → warn + no-op",
			envActionID:   "1",
			seedStatus:    db.ActionStatusPending,
			stdin:         `{"hook_event_name":"SessionStart"}`,
			wantStderrSub: "missing session_id",
			wantUnchanged: true,
		},
		{
			name:          "action not found → warn + no-op",
			envActionID:   "9999",
			seedStatus:    db.ActionStatusPending,
			stdin:         validPayload,
			wantStderrSub: "not found",
			wantUnchanged: true,
		},
		{
			name:          "terminal status (done) → skip",
			envActionID:   "1",
			seedStatus:    db.ActionStatusDone,
			stdin:         validPayload,
			wantStderrSub: "is done, skipping",
			wantUnchanged: true,
		},
		{
			name:          "terminal status (failed) → skip",
			envActionID:   "1",
			seedStatus:    db.ActionStatusFailed,
			stdin:         validPayload,
			wantStderrSub: "is failed, skipping",
			wantUnchanged: true,
		},
		{
			name:          "terminal status (cancelled) → skip",
			envActionID:   "1",
			seedStatus:    db.ActionStatusCancelled,
			stdin:         validPayload,
			wantStderrSub: "is cancelled, skipping",
			wantUnchanged: true,
		},
		{
			name:          "pending action → metadata merged",
			envActionID:   "1",
			seedStatus:    db.ActionStatusPending,
			stdin:         validPayload,
			wantSessionID: "abc-123",
		},
		{
			name:          "running action → metadata merged",
			envActionID:   "1",
			seedStatus:    db.ActionStatusRunning,
			stdin:         validPayload,
			wantSessionID: "abc-123",
		},
		{
			name:          "running with prior different session_id → overwritten",
			envActionID:   "1",
			seedStatus:    db.ActionStatusRunning,
			seedMetadata:  `{"claude_session_id":"old-uuid","mode":"interactive"}`,
			stdin:         validPayload,
			wantSessionID: "abc-123",
		},
		{
			name:          "running with same session_id → idempotent no-op",
			envActionID:   "1",
			seedStatus:    db.ActionStatusRunning,
			seedMetadata:  `{"claude_session_id":"abc-123","mode":"interactive"}`,
			stdin:         validPayload,
			wantUnchanged: true,
			wantNoMerge:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)

			taskID, err := d.InsertTask(1, "test", "{}", "")
			if err != nil {
				t.Fatalf("insert task: %v", err)
			}
			meta := tc.seedMetadata
			if meta == "" {
				meta = "{}"
			}
			id, err := d.InsertAction("test", taskID, meta, tc.seedStatus, nil)
			if err != nil {
				t.Fatalf("insert action: %v", err)
			}

			if tc.envActionID == "" {
				t.Setenv("TQ_ACTION_ID", "")
			} else {
				t.Setenv("TQ_ACTION_ID", tc.envActionID)
			}

			stderr := new(bytes.Buffer)
			openStore := func() (db.Store, func(), error) {
				return d, func() {}, nil
			}
			runClaudeSessionRecord(strings.NewReader(tc.stdin), stderr, openStore)

			if tc.wantStderrSub != "" && !strings.Contains(stderr.String(), tc.wantStderrSub) {
				t.Errorf("stderr = %q, want substring %q", stderr.String(), tc.wantStderrSub)
			}
			if tc.wantStderrSub == "" && stderr.Len() != 0 {
				t.Errorf("expected silent stderr, got %q", stderr.String())
			}

			got, err := d.GetAction(id)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}

			gotSessionID := metaField(t, got.Metadata, dispatch.MetaKeyClaudeSessionID)
			seedSessionID := metaField(t, meta, dispatch.MetaKeyClaudeSessionID)

			if tc.wantNoMerge {
				events, err := d.ListEvents("action", id)
				if err != nil {
					t.Fatalf("list events: %v", err)
				}
				for _, e := range events {
					if e.EventType == "action.metadata_merged" {
						t.Errorf("expected no merge event, got %q", e.EventType)
					}
				}
			}

			if tc.wantUnchanged {
				if gotSessionID != seedSessionID {
					t.Errorf("metadata.claude_session_id changed: got %q, want unchanged %q", gotSessionID, seedSessionID)
				}
				return
			}

			if gotSessionID != tc.wantSessionID {
				t.Errorf("metadata.claude_session_id = %q, want %q", gotSessionID, tc.wantSessionID)
			}
		})
	}
}

func TestRunClaudeSessionRecord_ExecutorStamp(t *testing.T) {
	const validPayload = `{"session_id":"sess-cloud","hook_event_name":"SessionStart","source":"startup"}`

	tests := []struct {
		name          string
		envRemote     string
		seedMetadata  string
		wantExecutor  string
		wantSessionID string
	}{
		{
			name:          "CLAUDE_CODE_REMOTE=true → executor=cloud stamped",
			envRemote:     "true",
			wantExecutor:  dispatch.ExecutorCloud,
			wantSessionID: "sess-cloud",
		},
		{
			name:          "CLAUDE_CODE_REMOTE unset → executor not stamped",
			envRemote:     "",
			wantExecutor:  "",
			wantSessionID: "sess-cloud",
		},
		{
			name:          "CLAUDE_CODE_REMOTE=false → executor not stamped",
			envRemote:     "false",
			wantExecutor:  "",
			wantSessionID: "sess-cloud",
		},
		{
			name:          "CLAUDE_CODE_REMOTE=true with executor=cloud already set → idempotent (no merge event)",
			envRemote:     "true",
			seedMetadata:  `{"executor":"cloud","claude_session_id":"sess-cloud"}`,
			wantExecutor:  dispatch.ExecutorCloud,
			wantSessionID: "sess-cloud",
		},
		{
			name:          "CLAUDE_CODE_REMOTE=true alongside fresh session_id → both merged",
			envRemote:     "true",
			seedMetadata:  `{"mode":"interactive"}`,
			wantExecutor:  dispatch.ExecutorCloud,
			wantSessionID: "sess-cloud",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, err := d.InsertTask(1, "test", "{}", "")
			if err != nil {
				t.Fatalf("insert task: %v", err)
			}
			meta := tc.seedMetadata
			if meta == "" {
				meta = "{}"
			}
			id, err := d.InsertAction("test", taskID, meta, db.ActionStatusRunning, nil)
			if err != nil {
				t.Fatalf("insert action: %v", err)
			}

			t.Setenv("TQ_ACTION_ID", "1")
			t.Setenv("CLAUDE_CODE_REMOTE", tc.envRemote)

			stderr := new(bytes.Buffer)
			openStore := func() (db.Store, func(), error) {
				return d, func() {}, nil
			}
			runClaudeSessionRecord(strings.NewReader(validPayload), stderr, openStore)

			if stderr.Len() != 0 {
				t.Errorf("expected silent stderr, got %q", stderr.String())
			}

			got, err := d.GetAction(id)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}

			if gotSession := metaField(t, got.Metadata, dispatch.MetaKeyClaudeSessionID); gotSession != tc.wantSessionID {
				t.Errorf("metadata.claude_session_id = %q, want %q", gotSession, tc.wantSessionID)
			}
			if gotExecutor := metaField(t, got.Metadata, dispatch.MetaKeyExecutor); gotExecutor != tc.wantExecutor {
				t.Errorf("metadata.executor = %q, want %q", gotExecutor, tc.wantExecutor)
			}
		})
	}
}

func metaField(t *testing.T, raw, key string) string {
	t.Helper()
	if raw == "" || raw == "{}" {
		return ""
	}
	m := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("parse metadata: %v", err)
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
