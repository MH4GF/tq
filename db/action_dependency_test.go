package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

// dispatchable reports how many pending actions NextPending would consider
// runnable right now (dependency- and time-gated).
func dispatchable(t *testing.T, d *db.DB) int {
	t.Helper()
	pc, err := d.CountPendingByDispatch()
	if err != nil {
		t.Fatalf("CountPendingByDispatch: %v", err)
	}
	return pc.Dispatchable
}

func TestActionDependency_ActionBlockerStatus(t *testing.T) {
	tests := []struct {
		name         string
		targetStatus string
		wantUnblock  bool
	}{
		{"done unblocks", db.ActionStatusDone, true},
		{"failed blocks forever", db.ActionStatusFailed, false},
		{"cancelled blocks forever", db.ActionStatusCancelled, false},
		{"running blocks", db.ActionStatusRunning, false},
		{"dispatched blocks", db.ActionStatusDispatched, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, _ := d.InsertTask(1, "task", "{}", "")

			target, err := d.InsertAction("target", taskID, "{}", db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetActionStatusForTest(target, tt.targetStatus); err != nil {
				t.Fatal(err)
			}
			follower, err := d.InsertAction("follower", taskID, "{}", db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: target}}); err != nil {
				t.Fatal(err)
			}

			got := dispatchable(t, d)
			want := 0
			if tt.wantUnblock {
				want = 1
			}
			if got != want {
				t.Errorf("dispatchable = %d, want %d", got, want)
			}
		})
	}
}

func TestActionDependency_TaskBlockerStatus(t *testing.T) {
	tests := []struct {
		name        string
		taskStatus  string
		wantUnblock bool
	}{
		{"open blocks", db.TaskStatusOpen, false},
		{"done unblocks", db.TaskStatusDone, true},
		{"archived unblocks", db.TaskStatusArchived, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			ownTask, _ := d.InsertTask(1, "own", "{}", "")
			depTask, _ := d.InsertTask(1, "dep", "{}", "")

			follower, err := d.InsertAction("follower", ownTask, "{}", db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeTask, ID: depTask}}); err != nil {
				t.Fatal(err)
			}
			if tt.taskStatus != db.TaskStatusOpen {
				if err := d.UpdateTask(depTask, tt.taskStatus, ""); err != nil {
					t.Fatal(err)
				}
			}

			got := dispatchable(t, d)
			want := 0
			if tt.wantUnblock {
				want = 1
			}
			if got != want {
				t.Errorf("dispatchable = %d, want %d", got, want)
			}
		})
	}
}

func TestActionDependency_MultipleAND(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	a1, _ := d.InsertAction("dep1", taskID, "{}", db.ActionStatusPending, nil, "")
	a2, _ := d.InsertAction("dep2", taskID, "{}", db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(a1, db.ActionStatusRunning)
	_ = d.SetActionStatusForTest(a2, db.ActionStatusRunning)

	follower, _ := d.InsertAction("follower", taskID, "{}", db.ActionStatusPending, nil, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{
		{Type: db.DepTypeAction, ID: a1},
		{Type: db.DepTypeAction, ID: a2},
	}); err != nil {
		t.Fatal(err)
	}

	if got := dispatchable(t, d); got != 0 {
		t.Fatalf("both deps unsatisfied: dispatchable = %d, want 0", got)
	}
	if err := d.MarkDone(a1, "ok"); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 0 {
		t.Fatalf("one dep still unsatisfied: dispatchable = %d, want 0", got)
	}
	if err := d.MarkDone(a2, "ok"); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 1 {
		t.Fatalf("all deps satisfied: dispatchable = %d, want 1", got)
	}
}

func TestActionDependency_DispatchAfterAND(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	target, _ := d.InsertAction("target", taskID, "{}", db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(target, db.ActionStatusRunning)

	future := "2999-12-31 23:59:59"
	follower, _ := d.InsertAction("follower", taskID, "{}", db.ActionStatusPending, &future, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: target}}); err != nil {
		t.Fatal(err)
	}

	// Dependency unsatisfied AND time not reached.
	if got := dispatchable(t, d); got != 0 {
		t.Fatalf("dispatchable = %d, want 0 (dep + time)", got)
	}
	// Satisfy the dependency; time gate still blocks.
	if err := d.MarkDone(target, "ok"); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 0 {
		t.Fatalf("dispatchable = %d, want 0 (time still future)", got)
	}
}

func TestActionDependency_NoDepsRegression(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	if _, err := d.InsertAction("plain", taskID, "{}", db.ActionStatusPending, nil, ""); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 1 {
		t.Fatalf("no-deps action: dispatchable = %d, want 1", got)
	}

	future := "2999-12-31 23:59:59"
	if _, err := d.InsertAction("later", taskID, "{}", db.ActionStatusPending, &future, ""); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 1 {
		t.Fatalf("time-only gate unchanged: dispatchable = %d, want 1", got)
	}
}

func TestActionDependency_NextPendingEndToEnd(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")
	ctx := context.Background()

	target, _ := d.InsertAction("target", taskID, "{}", db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(target, db.ActionStatusRunning)
	follower, _ := d.InsertAction("follower", taskID, "{}", db.ActionStatusPending, nil, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: target}}); err != nil {
		t.Fatal(err)
	}

	got, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("NextPending returned action #%d, want nil (blocked)", got.ID)
	}

	if err := d.MarkDone(target, "ok"); err != nil {
		t.Fatal(err)
	}
	got, err = d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != follower {
		t.Fatalf("NextPending = %v, want action #%d after dep done", got, follower)
	}
}

func TestActionDependency_CycleDetection(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	a1, _ := d.InsertAction("a1", taskID, "{}", db.ActionStatusPending, nil, "")
	a2, _ := d.InsertAction("a2", taskID, "{}", db.ActionStatusPending, nil, "")
	a3, _ := d.InsertAction("a3", taskID, "{}", db.ActionStatusPending, nil, "")

	if err := d.AddActionDependencies(a1, []db.ActionDep{{Type: db.DepTypeAction, ID: a1}}); err == nil {
		t.Error("self-dependency should be rejected")
	}
	if err := d.AddActionDependencies(a2, []db.ActionDep{{Type: db.DepTypeAction, ID: a1}}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddActionDependencies(a1, []db.ActionDep{{Type: db.DepTypeAction, ID: a2}}); err == nil {
		t.Error("A->B then B->A should be rejected as a cycle")
	}
	// Multi-hop: a3 depends on a2 (which depends on a1). a1 -> a3 closes it.
	if err := d.AddActionDependencies(a3, []db.ActionDep{{Type: db.DepTypeAction, ID: a2}}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddActionDependencies(a1, []db.ActionDep{{Type: db.DepTypeAction, ID: a3}}); err == nil {
		t.Error("A->B->C then C->A should be rejected as a cycle")
	}
}

func TestActionDependency_MissingBlocker(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")
	a, _ := d.InsertAction("a", taskID, "{}", db.ActionStatusPending, nil, "")

	if err := d.AddActionDependencies(a, []db.ActionDep{{Type: db.DepTypeAction, ID: 99999}}); err == nil {
		t.Error("non-existent blocker action should be rejected")
	}
	if err := d.AddActionDependencies(a, []db.ActionDep{{Type: db.DepTypeTask, ID: 99999}}); err == nil {
		t.Error("non-existent blocker task should be rejected")
	}
}

func TestActionDependency_ClearAndReplace(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	t1, _ := d.InsertAction("t1", taskID, "{}", db.ActionStatusPending, nil, "")
	t2, _ := d.InsertAction("t2", taskID, "{}", db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(t1, db.ActionStatusFailed)
	_ = d.SetActionStatusForTest(t2, db.ActionStatusDone)

	follower, _ := d.InsertAction("follower", taskID, "{}", db.ActionStatusPending, nil, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: t1}}); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 0 {
		t.Fatalf("blocked by failed dep: dispatchable = %d, want 0", got)
	}

	// Clear removes the bad dependency entirely.
	if err := d.ClearActionDependencies(follower); err != nil {
		t.Fatal(err)
	}
	if got := dispatchable(t, d); got != 1 {
		t.Fatalf("after clear: dispatchable = %d, want 1", got)
	}

	// Replace: re-point to a satisfied dependency.
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: t2}}); err != nil {
		t.Fatal(err)
	}
	deps, err := d.ListActionDependencies(follower)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].ID != t2 || !deps[0].Satisfied {
		t.Fatalf("ListActionDependencies = %+v, want [{action #%d satisfied}]", deps, t2)
	}
	if got := dispatchable(t, d); got != 1 {
		t.Fatalf("after replace with satisfied dep: dispatchable = %d, want 1", got)
	}
}

// TestActionDependency_GateGoParity locks the two independent encodings of the
// dependency-satisfaction rule together: dependencyGatePredicate (SQL, drives
// dispatch via NextPending/CountPendingByDispatch) and depSatisfied (Go, drives
// the displayed Satisfied flag). For the full dep_type x blocker-status matrix
// it asserts the two verdicts are identical, so any future edit to one without
// the other fails CI. It does not hardcode per-case expectations beyond a
// sanity check — the lock is the SQL==Go equality itself.
func TestActionDependency_GateGoParity(t *testing.T) {
	// Action blockers go through SetActionStatusForTest (raw UPDATE, no
	// validation) so an unknown status exercises depSatisfied's default branch.
	actionStatuses := []string{
		db.ActionStatusPending,
		db.ActionStatusRunning,
		db.ActionStatusDispatched,
		db.ActionStatusDone,
		db.ActionStatusFailed,
		db.ActionStatusCancelled,
		"weird-unknown-status",
	}
	// open/done/archived are the only valid task statuses (db/task.go), enforced
	// by UpdateTask validation — this matrix is therefore exhaustive for tasks.
	taskStatuses := []string{
		db.TaskStatusOpen,
		db.TaskStatusDone,
		db.TaskStatusArchived,
	}

	for _, st := range actionStatuses {
		t.Run("action blocker "+st, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			blockerTask, _ := d.InsertTask(1, "blocker", "{}", "")
			followerTask, _ := d.InsertTask(1, "follower", "{}", "")

			blocker, err := d.InsertAction("blocker", blockerTask, "{}", db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.SetActionStatusForTest(blocker, st); err != nil {
				t.Fatal(err)
			}
			follower, err := d.InsertAction("follower", followerTask, "{}", db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: blocker}}); err != nil {
				t.Fatal(err)
			}

			gate, err := db.ExportDependencyGateAllows(d, follower)
			if err != nil {
				t.Fatalf("ExportDependencyGateAllows: %v", err)
			}
			goSat := db.ExportDepSatisfied(db.DepTypeAction, st)
			if gate != goSat {
				t.Errorf("parity drift for action blocker status %q: SQL gate allows=%v, Go depSatisfied=%v", st, gate, goSat)
			}
			if want := st == db.ActionStatusDone; gate != want {
				t.Errorf("action blocker status %q: verdict=%v, want %v", st, gate, want)
			}
		})
	}

	for _, st := range taskStatuses {
		t.Run("task blocker "+st, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			blockerTask, _ := d.InsertTask(1, "blocker", "{}", "")
			followerTask, _ := d.InsertTask(1, "follower", "{}", "")

			if st != db.TaskStatusOpen {
				if err := d.UpdateTask(blockerTask, st, ""); err != nil {
					t.Fatal(err)
				}
			}
			follower, err := d.InsertAction("follower", followerTask, "{}", db.ActionStatusPending, nil, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeTask, ID: blockerTask}}); err != nil {
				t.Fatal(err)
			}

			gate, err := db.ExportDependencyGateAllows(d, follower)
			if err != nil {
				t.Fatalf("ExportDependencyGateAllows: %v", err)
			}
			goSat := db.ExportDepSatisfied(db.DepTypeTask, st)
			if gate != goSat {
				t.Errorf("parity drift for task blocker status %q: SQL gate allows=%v, Go depSatisfied=%v", st, gate, goSat)
			}
			if want := st == db.TaskStatusDone || st == db.TaskStatusArchived; gate != want {
				t.Errorf("task blocker status %q: verdict=%v, want %v", st, gate, want)
			}
		})
	}
}

func TestActionDependency_ClaimPendingBypassesGate(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")
	ctx := context.Background()

	target, _ := d.InsertAction("target", taskID, "{}", db.ActionStatusPending, nil, "")
	_ = d.SetActionStatusForTest(target, db.ActionStatusFailed)
	follower, _ := d.InsertAction("follower", taskID, "{}", db.ActionStatusPending, nil, "")
	if err := d.AddActionDependencies(follower, []db.ActionDep{{Type: db.DepTypeAction, ID: target}}); err != nil {
		t.Fatal(err)
	}

	// Auto path is blocked.
	if got := dispatchable(t, d); got != 0 {
		t.Fatalf("dispatchable = %d, want 0", got)
	}
	// Manual override claims it anyway.
	a, err := d.ClaimPending(ctx, follower)
	if err != nil {
		t.Fatalf("ClaimPending should bypass the dependency gate: %v", err)
	}
	if a.Status != db.ActionStatusRunning {
		t.Fatalf("ClaimPending status = %s, want running", a.Status)
	}
}

func TestInsertActionWithDependencies_AtomicOnDepFailure(t *testing.T) {
	tests := []struct {
		name    string
		deps    []db.ActionDep
		wantErr string
	}{
		{
			name:    "missing action blocker",
			deps:    []db.ActionDep{{Type: db.DepTypeAction, ID: 99999}},
			wantErr: "blocked-by action #99999 not found",
		},
		{
			name:    "missing task blocker",
			deps:    []db.ActionDep{{Type: db.DepTypeTask, ID: 99999}},
			wantErr: "blocked-by task #99999 not found",
		},
		{
			name:    "invalid dep type",
			deps:    []db.ActionDep{{Type: "bogus", ID: 1}},
			wantErr: `invalid dependency type "bogus"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := testutil.NewTestDB(t)
			testutil.SeedTestProjects(t, d)
			taskID, _ := d.InsertTask(1, "task", "{}", "")

			before := dispatchable(t, d)

			id, err := d.InsertActionWithDependencies(db.ActionInsertSpec{
				Title:    "follower",
				TaskID:   taskID,
				Metadata: "{}",
				Status:   db.ActionStatusPending,
			}, tt.deps)
			if err == nil {
				t.Fatalf("InsertActionWithDependencies returned id=%d, want error %q", id, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
			}
			if id != 0 {
				t.Errorf("returned id = %d, want 0 on failure", id)
			}

			if got := dispatchable(t, d); got != before {
				t.Errorf("dispatchable after rollback = %d, want %d (no orphan)", got, before)
			}

			actions, err := d.ListActions(db.ActionStatusPending, &taskID, 100)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range actions {
				if a.Title == "follower" {
					t.Errorf("orphan action #%d (title=follower) survived rollback", a.ID)
				}
			}
		})
	}
}

func TestInsertActionWithDependencies_BlockerGatesDispatch(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")
	ctx := context.Background()

	blocker, err := d.InsertAction("blocker", taskID, "{}", db.ActionStatusPending, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = d.SetActionStatusForTest(blocker, db.ActionStatusRunning)

	follower, err := d.InsertActionWithDependencies(db.ActionInsertSpec{
		Title:    "follower",
		TaskID:   taskID,
		Metadata: "{}",
		Status:   db.ActionStatusPending,
	}, []db.ActionDep{{Type: db.DepTypeAction, ID: blocker}})
	if err != nil {
		t.Fatalf("InsertActionWithDependencies: %v", err)
	}

	got, err := d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("NextPending returned action #%d, want nil (follower #%d should be gated)", got.ID, follower)
	}

	deps, err := d.ListActionDependencies(follower)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 1 || deps[0].ID != blocker {
		t.Fatalf("ListActionDependencies = %+v, want [{action #%d}]", deps, blocker)
	}

	if err := d.MarkDone(blocker, "ok"); err != nil {
		t.Fatal(err)
	}
	got, err = d.NextPending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != follower {
		t.Fatalf("NextPending = %v, want action #%d after blocker done", got, follower)
	}
}

func TestInsertActionWithDependencies_NoDeps(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, _ := d.InsertTask(1, "task", "{}", "")

	id, err := d.InsertActionWithDependencies(db.ActionInsertSpec{
		Title:    "plain",
		TaskID:   taskID,
		Metadata: "{}",
		Status:   db.ActionStatusPending,
	}, nil)
	if err != nil {
		t.Fatalf("InsertActionWithDependencies: %v", err)
	}
	if id == 0 {
		t.Fatal("returned id = 0")
	}
	if got := dispatchable(t, d); got != 1 {
		t.Fatalf("dispatchable = %d, want 1", got)
	}
}
