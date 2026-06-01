package cmd_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/MH4GF/tq/cmd"
	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/dispatch"
	"github.com/MH4GF/tq/testutil"
)

type mockWorker struct {
	result string
	err    error
}

func (m *mockWorker) Execute(_ context.Context, _ string, _ dispatch.ActionConfig, _ string, _, _ int64) (string, error) {
	return m.result, m.err
}

func TestDispatch_RecordsDaemonShortForLocalModes(t *testing.T) {
	tests := []struct {
		name string
		mode string
	}{
		{name: "default mode", mode: ""},
		{name: "interactive mode", mode: "interactive"},
		{name: "noninteractive mode", mode: "noninteractive"},
		{name: "legacy experimental_bg metadata is migrated to interactive at dispatch", mode: "experimental_bg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			cmd.SetDB(d)
			cmd.ResetForTest()
			cmd.SetConfigDir(t.TempDir())

			taskID, _ := d.InsertTask(1, "t", "{}", "")
			meta := `{"instruction":"x"}`
			if tc.mode != "" && tc.mode != "experimental_bg" {
				meta = `{"instruction":"x","mode":"` + tc.mode + `"}`
			}
			if tc.mode == "experimental_bg" {
				_, _ = d.InsertAction("t", taskID, `{"instruction":"x","mode":"experimental_bg"}`, db.ActionStatusPending, nil, "")
				if err := d.Migrate(); err != nil {
					t.Fatalf("migrate: %v", err)
				}
			} else {
				_, _ = d.InsertAction("t", taskID, meta, db.ActionStatusPending, nil, "")
			}

			worker := &mockWorker{result: "239007b1"}
			cmd.SetBgWorkerFactory(func() dispatch.Worker { return worker })
			t.Cleanup(func() { cmd.SetBgWorkerFactory(nil) })

			root := cmd.GetRootCmd()
			buf := new(bytes.Buffer)
			root.SetOut(buf)
			root.SetErr(buf)
			root.SetArgs([]string{"action", "dispatch", "1"})

			if err := root.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}

			a, err := d.GetAction(1)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if a.Status != db.ActionStatusRunning {
				t.Errorf("status = %q, want %q (bg lifecycle is driven by reaper)", a.Status, db.ActionStatusRunning)
			}
			meta2, err := dispatch.ParseActionMetadata(a.Metadata)
			if err != nil {
				t.Fatalf("parse metadata: %v", err)
			}
			if got, _ := meta2[dispatch.MetaKeyDaemonShort].(string); got != "239007b1" {
				t.Errorf("metadata.daemon_short = %q, want %q", got, "239007b1")
			}
			if !bytes.Contains(buf.Bytes(), []byte("239007b1")) {
				t.Errorf("stdout = %q, want to include daemon short id", buf.String())
			}
		})
	}
}

func TestDispatch_NoArgsErrors(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetConfigDir(t.TempDir())

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch"})

	if err := root.Execute(); err == nil {
		t.Fatalf("expected error for missing args, got nil")
	}
}

func TestDispatch_UnknownActionIDErrors(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetConfigDir(t.TempDir())

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch", "999"})

	err := root.Execute()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want substring %q", err.Error(), "not found")
	}
}

func TestDispatch_WorkerErrorMarksFailed(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	cmd.SetDB(d)
	cmd.ResetForTest()
	cmd.SetConfigDir(t.TempDir())

	taskID, _ := d.InsertTask(1, "t", "{}", "")
	d.InsertAction("t", taskID, `{"instruction":"x","mode":"interactive"}`, db.ActionStatusPending, nil, "")

	worker := &mockWorker{err: context.DeadlineExceeded}
	cmd.SetBgWorkerFactory(func() dispatch.Worker { return worker })
	t.Cleanup(func() { cmd.SetBgWorkerFactory(nil) })

	root := cmd.GetRootCmd()
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"action", "dispatch", "1"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "failed") {
		t.Errorf("output = %q, want substring %q", out, "failed")
	}
	a, _ := d.GetAction(1)
	if a.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", a.Status, db.ActionStatusFailed)
	}
}
