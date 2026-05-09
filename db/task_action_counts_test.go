package db_test

import (
	"sort"
	"testing"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

type countRow struct {
	taskID int64
	status string
	count  int64
}

func dumpCounts(t *testing.T, d *db.DB) []countRow {
	t.Helper()
	rows, err := d.Query("SELECT task_id, status, count FROM task_action_counts WHERE count > 0 ORDER BY task_id, status")
	if err != nil {
		t.Fatalf("query counts: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []countRow
	for rows.Next() {
		var r countRow
		if err := rows.Scan(&r.taskID, &r.status, &r.count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func dumpGroupBy(t *testing.T, d *db.DB) []countRow {
	t.Helper()
	rows, err := d.Query("SELECT task_id, status, COUNT(*) FROM actions GROUP BY task_id, status ORDER BY task_id, status")
	if err != nil {
		t.Fatalf("query groupby: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []countRow
	for rows.Next() {
		var r countRow
		if err := rows.Scan(&r.taskID, &r.status, &r.count); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return out
}

func equalCountRows(a, b []countRow) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestTaskActionCounts_TriggersStaySynced(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, err := d.InsertTask(1, "test", "{}", "")
	if err != nil {
		t.Fatalf("InsertTask: %v", err)
	}

	a1, err := d.InsertAction("a1", taskID, "{}", db.ActionStatusPending, nil)
	if err != nil {
		t.Fatalf("InsertAction a1: %v", err)
	}
	a2, err := d.InsertAction("a2", taskID, "{}", db.ActionStatusPending, nil)
	if err != nil {
		t.Fatalf("InsertAction a2: %v", err)
	}
	a3, err := d.InsertAction("a3", taskID, "{}", db.ActionStatusRunning, nil)
	if err != nil {
		t.Fatalf("InsertAction a3: %v", err)
	}

	type step struct {
		name       string
		mutate     func() error
		wantCounts map[string]int64
	}

	steps := []step{
		{
			name: "after inserts: 2 pending + 1 running",
			mutate: func() error {
				return nil
			},
			wantCounts: map[string]int64{
				db.ActionStatusPending: 2,
				db.ActionStatusRunning: 1,
			},
		},
		{
			name: "a1 pending → running",
			mutate: func() error {
				_, err := d.Exec("UPDATE actions SET status = ? WHERE id = ?", db.ActionStatusRunning, a1)
				return err
			},
			wantCounts: map[string]int64{
				db.ActionStatusPending: 1,
				db.ActionStatusRunning: 2,
			},
		},
		{
			name: "a1 running → dispatched",
			mutate: func() error {
				return d.MarkDispatched(a1)
			},
			wantCounts: map[string]int64{
				db.ActionStatusPending:    1,
				db.ActionStatusRunning:    1,
				db.ActionStatusDispatched: 1,
			},
		},
		{
			name: "a1 dispatched → running (resume)",
			mutate: func() error {
				_, err := d.Exec("UPDATE actions SET status = ? WHERE id = ?", db.ActionStatusRunning, a1)
				return err
			},
			wantCounts: map[string]int64{
				db.ActionStatusPending: 1,
				db.ActionStatusRunning: 2,
			},
		},
		{
			name: "a3 running → done via MarkDone",
			mutate: func() error {
				return d.MarkDone(a3, "ok")
			},
			wantCounts: map[string]int64{
				db.ActionStatusPending: 1,
				db.ActionStatusRunning: 1,
				db.ActionStatusDone:    1,
			},
		},
		{
			name: "a1 running → failed",
			mutate: func() error {
				return d.MarkFailed(a1, "boom")
			},
			wantCounts: map[string]int64{
				db.ActionStatusPending: 1,
				db.ActionStatusFailed:  1,
				db.ActionStatusDone:    1,
			},
		},
		{
			name: "a2 pending → cancelled",
			mutate: func() error {
				return d.MarkCancelled(a2, "stop")
			},
			wantCounts: map[string]int64{
				db.ActionStatusCancelled: 1,
				db.ActionStatusFailed:    1,
				db.ActionStatusDone:      1,
			},
		},
		{
			name: "DELETE a3 (done)",
			mutate: func() error {
				_, err := d.Exec("DELETE FROM actions WHERE id = ?", a3)
				return err
			},
			wantCounts: map[string]int64{
				db.ActionStatusCancelled: 1,
				db.ActionStatusFailed:    1,
			},
		},
		{
			name: "no-op UPDATE on title does not touch counts",
			mutate: func() error {
				_, err := d.Exec("UPDATE actions SET title = 'renamed' WHERE id = ?", a1)
				return err
			},
			wantCounts: map[string]int64{
				db.ActionStatusCancelled: 1,
				db.ActionStatusFailed:    1,
			},
		},
	}

	for _, st := range steps {
		t.Run(st.name, func(t *testing.T) {
			if err := st.mutate(); err != nil {
				t.Fatalf("mutate: %v", err)
			}
			gotByStatus := map[string]int64{}
			for _, r := range dumpCounts(t, d) {
				if r.taskID != taskID {
					t.Errorf("unexpected task_id %d", r.taskID)
				}
				gotByStatus[r.status] = r.count
			}
			if len(gotByStatus) != len(st.wantCounts) {
				t.Errorf("got %d statuses, want %d (got=%v want=%v)", len(gotByStatus), len(st.wantCounts), gotByStatus, st.wantCounts)
			}
			for status, want := range st.wantCounts {
				if got := gotByStatus[status]; got != want {
					t.Errorf("status=%q: got %d, want %d", status, got, want)
				}
			}

			// Cross-check: trigger-maintained counts must equal GROUP BY of actions.
			groupBy := dumpGroupBy(t, d)
			counts := dumpCounts(t, d)
			sort.Slice(groupBy, func(i, j int) bool {
				if groupBy[i].taskID != groupBy[j].taskID {
					return groupBy[i].taskID < groupBy[j].taskID
				}
				return groupBy[i].status < groupBy[j].status
			})
			sort.Slice(counts, func(i, j int) bool {
				if counts[i].taskID != counts[j].taskID {
					return counts[i].taskID < counts[j].taskID
				}
				return counts[i].status < counts[j].status
			})
			if !equalCountRows(groupBy, counts) {
				t.Errorf("counts diverge from GROUP BY actions:\ngroup_by=%v\ncounts=%v", groupBy, counts)
			}
		})
	}
}

func TestTaskActionCounts_GetTaskActionCount(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, err := d.InsertTask(1, "test", "{}", "")
	if err != nil {
		t.Fatalf("InsertTask: %v", err)
	}
	for range 3 {
		if _, err := d.InsertAction("p", taskID, "{}", db.ActionStatusPending, nil); err != nil {
			t.Fatalf("InsertAction: %v", err)
		}
	}
	for range 2 {
		if _, err := d.InsertAction("r", taskID, "{}", db.ActionStatusRunning, nil); err != nil {
			t.Fatalf("InsertAction: %v", err)
		}
	}
	if _, err := d.InsertAction("d", taskID, "{}", db.ActionStatusDone, nil); err != nil {
		t.Fatalf("InsertAction: %v", err)
	}

	tests := []struct {
		name     string
		statuses []string
		want     int64
	}{
		{"pending only", []string{db.ActionStatusPending}, 3},
		{"running only", []string{db.ActionStatusRunning}, 2},
		{"pending + running", []string{db.ActionStatusPending, db.ActionStatusRunning}, 5},
		{"all five", []string{db.ActionStatusPending, db.ActionStatusRunning, db.ActionStatusDone, db.ActionStatusFailed, db.ActionStatusCancelled}, 6},
		{"unknown status", []string{"nope"}, 0},
		{"empty list", nil, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.GetTaskActionCount(taskID, tc.statuses)
			if err != nil {
				t.Fatalf("GetTaskActionCount: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTaskActionCounts_BackfillIdempotent(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, err := d.InsertTask(1, "test", "{}", "")
	if err != nil {
		t.Fatalf("InsertTask: %v", err)
	}
	for range 5 {
		if _, err := d.InsertAction("p", taskID, "{}", db.ActionStatusPending, nil); err != nil {
			t.Fatalf("InsertAction: %v", err)
		}
	}
	for range 3 {
		if _, err := d.InsertAction("d", taskID, "{}", db.ActionStatusDone, nil); err != nil {
			t.Fatalf("InsertAction: %v", err)
		}
	}

	before := dumpCounts(t, d)
	if err := d.Migrate(); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if err := d.Migrate(); err != nil {
		t.Fatalf("third Migrate: %v", err)
	}
	after := dumpCounts(t, d)
	if !equalCountRows(before, after) {
		t.Errorf("counts changed across Migrate calls:\nbefore=%v\nafter=%v", before, after)
	}
}

func TestTaskActionCounts_BackfillFromExisting(t *testing.T) {
	d := testutil.NewTestDB(t)
	testutil.SeedTestProjects(t, d)
	taskID, err := d.InsertTask(1, "test", "{}", "")
	if err != nil {
		t.Fatalf("InsertTask: %v", err)
	}

	// Drop triggers so subsequent inserts do not auto-populate counts.
	for _, name := range []string{"trg_actions_count_insert", "trg_actions_count_update", "trg_actions_count_delete"} {
		if _, err := d.Exec("DROP TRIGGER IF EXISTS " + name); err != nil {
			t.Fatalf("drop trigger %s: %v", name, err)
		}
	}
	if _, err := d.Exec("DELETE FROM task_action_counts"); err != nil {
		t.Fatalf("clear counts: %v", err)
	}

	// Insert actions with no trigger active.
	for range 4 {
		if _, err := d.InsertAction("p", taskID, "{}", db.ActionStatusPending, nil); err != nil {
			t.Fatalf("InsertAction: %v", err)
		}
	}
	for range 2 {
		if _, err := d.InsertAction("r", taskID, "{}", db.ActionStatusRunning, nil); err != nil {
			t.Fatalf("InsertAction: %v", err)
		}
	}
	if rows := dumpCounts(t, d); len(rows) != 0 {
		t.Fatalf("counts non-empty before Migrate: %v", rows)
	}

	// Re-run Migrate — recreates triggers, sees empty table, runs backfill.
	if err := d.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	got := dumpCounts(t, d)
	want := dumpGroupBy(t, d)
	if !equalCountRows(got, want) {
		t.Errorf("backfill mismatch:\ngot=%v\nwant=%v", got, want)
	}
}
