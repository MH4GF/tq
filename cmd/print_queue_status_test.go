package cmd_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

// errorStore wraps a real db.Store and forces specific QueryReader methods
// used by printQueueStatus to return injected errors. Other calls pass
// through, so action creation still succeeds.
type errorStore struct {
	db.Store
	countPendingErr      error
	isDispatchEnabledErr error
	countRunningErr      error
}

func (s *errorStore) CountPendingByDispatch() (db.PendingCounts, error) {
	if s.countPendingErr != nil {
		return db.PendingCounts{}, s.countPendingErr
	}
	return s.Store.CountPendingByDispatch()
}

func (s *errorStore) IsActionDispatchEnabled(actionID int64) (bool, error) {
	if s.isDispatchEnabledErr != nil {
		return false, s.isDispatchEnabledErr
	}
	return s.Store.IsActionDispatchEnabled(actionID)
}

func (s *errorStore) CountRunningInteractive() (int, error) {
	if s.countRunningErr != nil {
		return 0, s.countRunningErr
	}
	return s.Store.CountRunningInteractive()
}

func TestPrintQueueStatus_DBErrors(t *testing.T) {
	tests := []struct {
		name      string
		injectErr func(*errorStore)
	}{
		{
			name: "CountPendingByDispatch fails",
			injectErr: func(s *errorStore) {
				s.countPendingErr = errors.New("pending count boom")
			},
		},
		{
			name: "IsActionDispatchEnabled fails",
			injectErr: func(s *errorStore) {
				s.isDispatchEnabledErr = errors.New("dispatch enabled boom")
			},
		},
		{
			name: "CountRunningInteractive fails",
			injectErr: func(s *errorStore) {
				s.countRunningErr = errors.New("running count boom")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			real := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, real)
			real.InsertTask(1, "test task", "{}", "")
			// Heartbeat so GetWorkerMaxInteractive returns a row and the
			// CountRunningInteractive branch is exercised.
			if err := real.UpdateWorkerHeartbeat(1); err != nil {
				t.Fatal(err)
			}

			store := &errorStore{Store: real}
			tc.injectErr(store)

			cmd.SetDB(store)
			cmd.ResetForTest()
			cmd.SetConfigDir(t.TempDir())

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "create", "do something", "--task", "1", "--title", "test"})

			if err := root.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			out := buf.String()
			if !strings.Contains(out, "status unavailable") {
				t.Errorf("output missing 'status unavailable':\n%s", out)
			}
			for _, banned := range []string{"0 pending", "interactive: 0/"} {
				if strings.Contains(out, banned) {
					t.Errorf("output should not contain fabricated %q hint:\n%s", banned, out)
				}
			}
		})
	}
}
