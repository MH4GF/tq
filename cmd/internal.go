package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
)

var internalCmd = &cobra.Command{
	Use:    "internal",
	Short:  "Internal commands invoked by Claude Code hooks (not for direct use)",
	Hidden: true,
	// Skip the root PersistentPreRunE so a hook firing on every claude session
	// (including manual non-tq sessions) does not pay the DB open + Migrate cost.
	// The leaf command opens the DB lazily after env-gating.
	PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
}

var claudeSessionRecordCmd = &cobra.Command{
	Use:   "claude-session-record",
	Short: "Record claude session_id from a SessionStart hook payload",
	Long: `Reads a Claude Code SessionStart hook payload from stdin and merges
session_id into the action metadata identified by the TQ_ACTION_ID env var.

When TQ_ACTION_ID is unset, this command exits silently — manual claude
invocations remain untouched. Any failure is logged to stderr but never
propagates a non-zero exit, so a misbehaving hook cannot abort the session.`,
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		runClaudeSessionRecord(cmd.InOrStdin(), cmd.ErrOrStderr(), openStoreFn)
	},
}

type sessionStartPayload struct {
	SessionID string `json:"session_id"`
}

// openStoreFn is overridable for tests; production opens the DB lazily so the
// unset-env path costs zero IO.
var openStoreFn = func() (db.Store, func(), error) {
	if database != nil {
		return database, func() {}, nil
	}
	dbPath, err := resolveDBPath()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve db path: %w", err)
	}
	if err := ensureLocalDBDir(dbPath); err != nil {
		return nil, nil, err
	}
	d, err := db.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}
	if err := d.Migrate(); err != nil {
		_ = d.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	return d, func() { _ = d.Close() }, nil
}

func runClaudeSessionRecord(stdin io.Reader, stderr io.Writer, openStore func() (db.Store, func(), error)) {
	actionIDStr := os.Getenv("TQ_ACTION_ID")
	if actionIDStr == "" {
		return
	}

	actionID, err := strconv.ParseInt(actionIDStr, 10, 64)
	if err != nil {
		warnf(stderr, "invalid TQ_ACTION_ID %q: %v", actionIDStr, err)
		return
	}

	raw, err := io.ReadAll(stdin)
	if err != nil {
		warnf(stderr, "read stdin: %v", err)
		return
	}

	var payload sessionStartPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		warnf(stderr, "parse payload: %v", err)
		return
	}
	if payload.SessionID == "" {
		warnf(stderr, "payload missing session_id")
		return
	}

	store, closeStore, err := openStore()
	if err != nil {
		warnf(stderr, "%v", err)
		return
	}
	defer closeStore()

	action, err := store.GetAction(actionID)
	if err != nil {
		warnf(stderr, "action #%d not found: %v", actionID, err)
		return
	}
	if db.IsTerminalActionStatus(action.Status) {
		warnf(stderr, "action #%d is %s, skipping", actionID, action.Status)
		return
	}
	merge := map[string]any{}
	// Idempotent: SessionStart fires on resume/clear/compact with the same
	// session_id; skip the write to avoid redundant action.metadata_merged events.
	if !metadataHasSessionID(action.Metadata, payload.SessionID) {
		merge[dispatch.MetaKeyClaudeSessionID] = payload.SessionID
	}
	// CLAUDE_CODE_REMOTE=true is set by Claude Code when running in cloud
	// (Claude Code on the web, including Cloud Routines). Stamp executor=cloud
	// so the local reaper knows this action is not its responsibility.
	if os.Getenv("CLAUDE_CODE_REMOTE") == "true" &&
		!dispatch.MetadataHasValue(action.Metadata, dispatch.MetaKeyExecutor, dispatch.ExecutorCloud) {
		merge[dispatch.MetaKeyExecutor] = dispatch.ExecutorCloud
	}
	if len(merge) == 0 {
		return
	}

	if err := store.MergeActionMetadata(actionID, merge); err != nil {
		warnf(stderr, "merge metadata: %v", err)
		return
	}
}

func metadataHasSessionID(rawMetadata, expected string) bool {
	if rawMetadata == "" || rawMetadata == "{}" {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rawMetadata), &m); err != nil {
		return false
	}
	v, ok := m[dispatch.MetaKeyClaudeSessionID].(string)
	return ok && v == expected
}

func warnf(stderr io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(stderr, "tq claude-session-record: "+format+"\n", args...)
}

func init() {
	internalCmd.AddCommand(claudeSessionRecordCmd)
	rootCmd.AddCommand(internalCmd)
}
