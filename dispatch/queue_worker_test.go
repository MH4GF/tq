package dispatch

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MH4GF/tq/db"
	"github.com/MH4GF/tq/testutil"
)

func ptr[T any](v T) *T { return &v }

func withShortDeferBackoff(t *testing.T) {
	t.Helper()
	original := defaultDeferBackoff
	defaultDeferBackoff = 50 * time.Millisecond
	t.Cleanup(func() { defaultDeferBackoff = original })
}

type bgWorkerStub struct {
	mu     sync.Mutex
	count  int
	short  string
	err    error
	gotCfg ActionConfig
}

func (w *bgWorkerStub) Execute(_ context.Context, _ string, cfg ActionConfig, _ string, _, _ int64) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.count++
	w.gotCfg = cfg
	if w.err != nil {
		return "", w.err
	}
	return w.short, nil
}

type fakeBgStateReader struct {
	mu     sync.Mutex
	states map[string][]byte
	errs   map[string]error
}

func (r *fakeBgStateReader) set(short string, payload []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.states == nil {
		r.states = make(map[string][]byte)
	}
	r.states[short] = payload
}

func (r *fakeBgStateReader) setErr(short string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.errs == nil {
		r.errs = make(map[string]error)
	}
	r.errs[short] = err
}

func (r *fakeBgStateReader) ReadState(short string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err, ok := r.errs[short]; ok {
		return nil, err
	}
	payload, ok := r.states[short]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: short, Err: fs.ErrNotExist}
	}
	return payload, nil
}

func setupTaskAndAction(t *testing.T, meta string) (db.Store, int64) {
	t.Helper()
	d := testutil.NewTestDB(t)
	projectID, err := d.InsertProject("p", "", "{}")
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	taskID, err := d.InsertTask(projectID, "t", "{}", "")
	if err != nil {
		t.Fatalf("insert task: %v", err)
	}
	actionID, err := d.InsertAction("a", taskID, meta, db.ActionStatusPending, nil, "")
	if err != nil {
		t.Fatalf("insert action: %v", err)
	}
	return d, actionID
}

func claimRunning(t *testing.T, d db.Store, actionID int64) {
	t.Helper()
	if _, err := d.ClaimPending(context.Background(), actionID); err != nil {
		t.Fatalf("claim pending: %v", err)
	}
}

func TestReapBg(t *testing.T) {
	notFound := &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist}
	cases := []struct {
		name              string
		metadata          string
		daemonShort       string
		startedAtAgo      time.Duration
		readerPayload     string
		readerErr         error
		grace             time.Duration
		hardTimeout       time.Duration
		expectStatus      string
		expectResult      string
		expectMetadataSub string
	}{
		{
			name:              "marks done on terminal state",
			metadata:          `{"instruction":"x","mode":"interactive","daemon_short":"aaaaaaaa"}`,
			daemonShort:       "aaaaaaaa",
			readerPayload:     `{"state":"done","sessionId":"sess-1","output":{"result":"all good"}}`,
			grace:             time.Hour,
			expectStatus:      db.ActionStatusDone,
			expectResult:      "all good",
			expectMetadataSub: `"claude_session_id":"sess-1"`,
		},
		{
			name:          "marks failed on terminal state",
			metadata:      `{"instruction":"x","mode":"noninteractive","daemon_short":"bbbbbbbb"}`,
			daemonShort:   "bbbbbbbb",
			readerPayload: `{"state":"failed","detail":"boom"}`,
			grace:         time.Hour,
			expectStatus:  db.ActionStatusFailed,
			expectResult:  "boom",
		},
		{
			name:          "legacy experimental_bg is handled",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"cccccccc"}`,
			daemonShort:   "cccccccc",
			readerPayload: `{"state":"done","output":{"result":"legacy ok"}}`,
			grace:         time.Hour,
			expectStatus:  db.ActionStatusDone,
		},
		{
			name:          "skips cloud executor actions",
			metadata:      `{"instruction":"x","mode":"interactive","daemon_short":"dddddddd","executor":"cloud"}`,
			daemonShort:   "dddddddd",
			readerPayload: `{"state":"done"}`,
			grace:         time.Hour,
			expectStatus:  db.ActionStatusRunning,
		},
		{
			name:         "missing job after grace marks failed",
			metadata:     `{"instruction":"x","mode":"interactive","daemon_short":"eeeeeeee"}`,
			daemonShort:  "eeeeeeee",
			startedAtAgo: 2 * time.Hour,
			readerErr:    notFound,
			grace:        time.Minute,
			expectStatus: db.ActionStatusFailed,
			expectResult: "daemon job dir missing",
		},
		{
			name:         "missing job within grace is kept",
			metadata:     `{"instruction":"x","mode":"interactive","daemon_short":"ffffffff"}`,
			daemonShort:  "ffffffff",
			readerErr:    notFound,
			grace:        time.Hour,
			expectStatus: db.ActionStatusRunning,
		},
		{
			name:          "noninteractive wedged hits hard timeout",
			metadata:      `{"instruction":"x","mode":"noninteractive","daemon_short":"abcd1234"}`,
			daemonShort:   "abcd1234",
			startedAtAgo:  5 * time.Hour,
			readerPayload: `{"state":"working"}`,
			grace:         time.Minute,
			hardTimeout:   4 * time.Hour,
			expectStatus:  db.ActionStatusFailed,
			expectResult:  "hard timeout",
		},
		{
			name:          "interactive never hits hard timeout",
			metadata:      `{"instruction":"x","mode":"interactive","daemon_short":"abcd1235"}`,
			daemonShort:   "abcd1235",
			startedAtAgo:  100 * time.Hour,
			readerPayload: `{"state":"working"}`,
			grace:         time.Minute,
			hardTimeout:   4 * time.Hour,
			expectStatus:  db.ActionStatusRunning,
		},
		{
			name:          "legacy experimental_bg exempt from hard timeout",
			metadata:      `{"instruction":"x","mode":"experimental_bg","daemon_short":"abcd1236"}`,
			daemonShort:   "abcd1236",
			startedAtAgo:  100 * time.Hour,
			readerPayload: `{"state":"working"}`,
			grace:         time.Minute,
			hardTimeout:   4 * time.Hour,
			expectStatus:  db.ActionStatusRunning,
		},
		{
			name:          "missing mode treated as interactive for timeout",
			metadata:      `{"instruction":"x","daemon_short":"abcd1237"}`,
			daemonShort:   "abcd1237",
			startedAtAgo:  100 * time.Hour,
			readerPayload: `{"state":"working"}`,
			grace:         time.Minute,
			hardTimeout:   4 * time.Hour,
			expectStatus:  db.ActionStatusRunning,
		},
		{
			name:          "noninteractive done wins over timeout",
			metadata:      `{"instruction":"x","mode":"noninteractive","daemon_short":"abcd1238"}`,
			daemonShort:   "abcd1238",
			startedAtAgo:  10 * time.Hour,
			readerPayload: `{"state":"done","output":{"result":"late but ok"}}`,
			grace:         time.Minute,
			hardTimeout:   4 * time.Hour,
			expectStatus:  db.ActionStatusDone,
			expectResult:  "late but ok",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, actionID := setupTaskAndAction(t, tc.metadata)
			claimRunning(t, d, actionID)
			if tc.startedAtAgo > 0 {
				if err := d.SetActionTmuxInfoForTest(actionID, nil, nil, ptr(time.Now().Add(-tc.startedAtAgo))); err != nil {
					t.Fatalf("set started_at: %v", err)
				}
			}

			reader := &fakeBgStateReader{}
			if tc.readerPayload != "" {
				reader.set(tc.daemonShort, []byte(tc.readerPayload))
			}
			if tc.readerErr != nil {
				reader.setErr(tc.daemonShort, tc.readerErr)
			}

			cfg := WorkerConfig{
				DispatchConfig:              DispatchConfig{DB: d},
				BgStateReader:               reader,
				BgMissingJobGrace:           tc.grace,
				BgNonInteractiveHardTimeout: tc.hardTimeout,
			}

			reapBg(cfg, time.Now())

			updated, err := d.GetAction(actionID)
			if err != nil {
				t.Fatalf("get action: %v", err)
			}
			if updated.Status != tc.expectStatus {
				t.Errorf("status = %q, want %q", updated.Status, tc.expectStatus)
			}
			if tc.expectResult != "" {
				if !updated.Result.Valid || !strings.Contains(updated.Result.String, tc.expectResult) {
					t.Errorf("result = %q, want substring %q", updated.Result.String, tc.expectResult)
				}
			}
			if tc.expectMetadataSub != "" && !strings.Contains(updated.Metadata, tc.expectMetadataSub) {
				t.Errorf("metadata missing %q: %q", tc.expectMetadataSub, updated.Metadata)
			}
		})
	}
}

func TestReapOrphans(t *testing.T) {
	cases := []struct {
		name         string
		metadata     string
		expectStatus string
		expectResult string
	}{
		{
			name:         "marks running without daemon_short as failed",
			metadata:     `{"instruction":"x","mode":"interactive"}`,
			expectStatus: db.ActionStatusFailed,
			expectResult: "orphaned",
		},
		{
			name:         "skips cloud executor",
			metadata:     `{"instruction":"x","mode":"interactive","executor":"cloud"}`,
			expectStatus: db.ActionStatusRunning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, actionID := setupTaskAndAction(t, tc.metadata)
			claimRunning(t, d, actionID)
			if err := d.SetActionTmuxInfoForTest(actionID, nil, nil, ptr(time.Now().Add(-2*time.Hour))); err != nil {
				t.Fatalf("set started_at: %v", err)
			}

			cfg := WorkerConfig{
				DispatchConfig:    DispatchConfig{DB: d},
				BgMissingJobGrace: time.Minute,
			}

			reapOrphans(cfg, time.Now())

			updated, _ := d.GetAction(actionID)
			if updated.Status != tc.expectStatus {
				t.Errorf("status = %q, want %q", updated.Status, tc.expectStatus)
			}
			if tc.expectResult != "" && !strings.Contains(updated.Result.String, tc.expectResult) {
				t.Errorf("reason = %q, want substring %q", updated.Result.String, tc.expectResult)
			}
		})
	}
}

type failingMergeStore struct {
	db.Store
}

func (f *failingMergeStore) MergeActionMetadata(int64, map[string]any) error {
	return errors.New("simulated merge failure")
}

func TestQueueWorker_BgMergeFailureDoesNotStrandRunningAction(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive"}`)
	wrapped := &failingMergeStore{Store: d}

	bg := &bgWorkerStub{short: "abcd1234"}
	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:         wrapped,
			BgFunc:     func() Worker { return bg },
			RemoteFunc: func() Worker { return &bgWorkerStub{} },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 5,
		BgMissingJobGrace: 10 * time.Millisecond,
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne: %v", err)
	}
	if !dispatched {
		t.Fatal("expected dispatchOne to report success despite merge failure")
	}
	if bg.count != 1 {
		t.Errorf("bg worker called %d times, want 1", bg.count)
	}

	afterDispatch, _ := d.GetAction(actionID)
	if afterDispatch.Status != db.ActionStatusRunning {
		t.Fatalf("after dispatch, status = %q, want %q", afterDispatch.Status, db.ActionStatusRunning)
	}
	if strings.Contains(afterDispatch.Metadata, "daemon_short") {
		t.Fatalf("daemon_short unexpectedly persisted despite merge failure: %q", afterDispatch.Metadata)
	}

	if err := d.SetActionTmuxInfoForTest(actionID, nil, nil, ptr(time.Now().Add(-time.Hour))); err != nil {
		t.Fatalf("backdate started_at: %v", err)
	}

	for range 3 {
		reapStaleActions(cfg)
	}

	final, _ := d.GetAction(actionID)
	if final.Status != db.ActionStatusFailed {
		t.Errorf("after reap ticks, status = %q, want %q (orphan reaped)", final.Status, db.ActionStatusFailed)
	}
	if !strings.Contains(final.Result.String, "orphaned") {
		t.Errorf("reason = %q, want substring %q", final.Result.String, "orphaned")
	}
}

func TestSlotPredicates_LegacyExperimentalBgCountedInInteractive(t *testing.T) {
	d, _ := setupTaskAndAction(t, `{"instruction":"x","mode":"experimental_bg","daemon_short":"abcd1234"}`)
	projectID := int64(1)
	taskID, _ := d.InsertTask(projectID, "t2", "{}", "")
	id2, _ := d.InsertAction("a2", taskID, `{"instruction":"x","mode":"interactive"}`, db.ActionStatusPending, nil, "")
	id3, _ := d.InsertAction("a3", taskID, `{"instruction":"x","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	for _, id := range []int64{id2, id3} {
		if err := d.SetActionStatusForTest(id, db.ActionStatusRunning); err != nil {
			t.Fatalf("set status running: %v", err)
		}
	}
	if err := d.SetActionStatusForTest(1, db.ActionStatusRunning); err != nil {
		t.Fatalf("set status running for legacy: %v", err)
	}

	gotInter, err := d.CountRunningInteractive()
	if err != nil {
		t.Fatalf("CountRunningInteractive: %v", err)
	}
	if gotInter != 2 {
		t.Errorf("CountRunningInteractive = %d, want 2 (legacy bg + interactive)", gotInter)
	}

	gotNon, err := d.CountRunningNonInteractive()
	if err != nil {
		t.Fatalf("CountRunningNonInteractive: %v", err)
	}
	if gotNon != 1 {
		t.Errorf("CountRunningNonInteractive = %d, want 1", gotNon)
	}
}

func TestDispatchOne_DispatchesInteractiveViaBg(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"do","mode":"interactive"}`)

	bg := &bgWorkerStub{short: "abcd1234"}
	rem := &bgWorkerStub{}

	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:         d,
			BgFunc:     func() Worker { return bg },
			RemoteFunc: func() Worker { return rem },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 5,
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne: %v", err)
	}
	if !dispatched {
		t.Error("dispatchOne reported nothing dispatched")
	}
	if bg.count != 1 {
		t.Errorf("BgWorker called %d times, want 1", bg.count)
	}
	if rem.count != 0 {
		t.Errorf("RemoteWorker called %d times, want 0", rem.count)
	}
	if bg.gotCfg.Mode != ModeInteractive {
		t.Errorf("worker received mode %q, want %q", bg.gotCfg.Mode, ModeInteractive)
	}

	updated, _ := d.GetAction(actionID)
	if !strings.Contains(updated.Metadata, `"daemon_short":"abcd1234"`) {
		t.Errorf("daemon_short not merged: %q", updated.Metadata)
	}
}

func TestDispatchOne_DefersWhenSlotFull(t *testing.T) {
	withShortDeferBackoff(t)
	d := testutil.NewTestDB(t)
	projectID, _ := d.InsertProject("p", "", "{}")
	taskID, _ := d.InsertTask(projectID, "t", "{}", "")

	for range 3 {
		id, _ := d.InsertAction("blocking", taskID, `{"instruction":"x","mode":"interactive","daemon_short":"running01"}`, db.ActionStatusPending, nil, "")
		if err := d.SetActionStatusForTest(id, db.ActionStatusRunning); err != nil {
			t.Fatalf("set status running: %v", err)
		}
	}

	newID, _ := d.InsertAction("pending", taskID, `{"instruction":"do","mode":"interactive"}`, db.ActionStatusPending, nil, "")

	bg := &bgWorkerStub{short: "abcd1234"}
	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:         d,
			BgFunc:     func() Worker { return bg },
			RemoteFunc: func() Worker { return &bgWorkerStub{} },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 5,
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne: %v", err)
	}
	if dispatched {
		t.Error("expected dispatch deferred because slot pool full")
	}
	if bg.count != 0 {
		t.Errorf("BgWorker unexpectedly called %d times when deferred", bg.count)
	}

	updated, _ := d.GetAction(newID)
	if updated.Status != db.ActionStatusPending {
		t.Errorf("status = %q, want %q (deferred)", updated.Status, db.ActionStatusPending)
	}
}

func TestDispatchOne_SkipsDeferredHeadAndDispatchesNextPool(t *testing.T) {
	withShortDeferBackoff(t)
	d := testutil.NewTestDB(t)
	projectID, _ := d.InsertProject("p", "", "{}")
	taskID, _ := d.InsertTask(projectID, "t", "{}", "")

	for range 3 {
		id, _ := d.InsertAction("blocking", taskID, `{"instruction":"x","mode":"interactive","daemon_short":"running01"}`, db.ActionStatusPending, nil, "")
		if err := d.SetActionStatusForTest(id, db.ActionStatusRunning); err != nil {
			t.Fatalf("set status running: %v", err)
		}
	}

	deferredID, _ := d.InsertAction("interactive-pending", taskID, `{"instruction":"do","mode":"interactive"}`, db.ActionStatusPending, nil, "")
	openID, _ := d.InsertAction("noninteractive-pending", taskID, `{"instruction":"do","mode":"noninteractive"}`, db.ActionStatusPending, nil, "")

	bg := &bgWorkerStub{short: "abcd1234"}
	cfg := WorkerConfig{
		DispatchConfig: DispatchConfig{
			DB:         d,
			BgFunc:     func() Worker { return bg },
			RemoteFunc: func() Worker { return &bgWorkerStub{} },
		},
		MaxInteractive:    3,
		MaxNonInteractive: 5,
	}

	dispatched, err := dispatchOne(context.Background(), cfg)
	if err != nil {
		t.Fatalf("dispatchOne: %v", err)
	}
	if !dispatched {
		t.Fatal("expected dispatchOne to skip deferred interactive head and dispatch the noninteractive successor")
	}
	if bg.count != 1 {
		t.Errorf("BgWorker called %d times, want 1 (only the noninteractive should dispatch)", bg.count)
	}
	if bg.gotCfg.Mode != ModeNonInteractive {
		t.Errorf("dispatched mode = %q, want %q", bg.gotCfg.Mode, ModeNonInteractive)
	}

	deferred, _ := d.GetAction(deferredID)
	if deferred.Status != db.ActionStatusPending {
		t.Errorf("interactive head status = %q, want %q (deferred back to pending)", deferred.Status, db.ActionStatusPending)
	}
	if !deferred.DispatchAfter.Valid {
		t.Error("interactive head should have dispatch_after set after defer")
	}

	open, _ := d.GetAction(openID)
	if open.Status != db.ActionStatusRunning {
		t.Errorf("noninteractive successor status = %q, want %q", open.Status, db.ActionStatusRunning)
	}
}
