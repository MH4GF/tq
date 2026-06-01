package dispatch

import (
	"context"
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

func TestReapBg_MarksDoneOnTerminalState(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive","daemon_short":"aaaaaaaa"}`)
	claimRunning(t, d, actionID)

	reader := &fakeBgStateReader{}
	reader.set("aaaaaaaa", []byte(`{"state":"done","sessionId":"sess-1","output":{"result":"all good"}}`))

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Hour,
	}

	reapBg(cfg, time.Now())

	updated, err := d.GetAction(actionID)
	if err != nil {
		t.Fatalf("get action: %v", err)
	}
	if updated.Status != db.ActionStatusDone {
		t.Errorf("status = %q, want %q", updated.Status, db.ActionStatusDone)
	}
	if !updated.Result.Valid || updated.Result.String != "all good" {
		t.Errorf("result = %q, want %q", updated.Result.String, "all good")
	}
	if !strings.Contains(updated.Metadata, `"claude_session_id":"sess-1"`) {
		t.Errorf("claude_session_id not backfilled: %q", updated.Metadata)
	}
}

func TestReapBg_MarksFailedOnTerminalState(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"noninteractive","daemon_short":"bbbbbbbb"}`)
	claimRunning(t, d, actionID)

	reader := &fakeBgStateReader{}
	reader.set("bbbbbbbb", []byte(`{"state":"failed","detail":"boom"}`))

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Hour,
	}

	reapBg(cfg, time.Now())

	updated, _ := d.GetAction(actionID)
	if updated.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q", updated.Status, db.ActionStatusFailed)
	}
	if !updated.Result.Valid || updated.Result.String != "boom" {
		t.Errorf("result = %q, want %q", updated.Result.String, "boom")
	}
}

func TestReapBg_LegacyExperimentalBgIsHandled(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"experimental_bg","daemon_short":"cccccccc"}`)
	claimRunning(t, d, actionID)

	reader := &fakeBgStateReader{}
	reader.set("cccccccc", []byte(`{"state":"done","output":{"result":"legacy ok"}}`))

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Hour,
	}

	reapBg(cfg, time.Now())

	updated, _ := d.GetAction(actionID)
	if updated.Status != db.ActionStatusDone {
		t.Errorf("legacy experimental_bg not reaped: status = %q", updated.Status)
	}
}

func TestReapBg_SkipsCloudExecutorActions(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive","daemon_short":"dddddddd","executor":"cloud"}`)
	claimRunning(t, d, actionID)

	reader := &fakeBgStateReader{}
	reader.set("dddddddd", []byte(`{"state":"done"}`))

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Hour,
	}

	reapBg(cfg, time.Now())

	updated, _ := d.GetAction(actionID)
	if updated.Status != db.ActionStatusRunning {
		t.Errorf("cloud executor action should not be reaped, status = %q", updated.Status)
	}
}

func TestReapBg_MissingJobAfterGraceMarksFailed(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive","daemon_short":"eeeeeeee"}`)
	claimRunning(t, d, actionID)
	if err := d.SetActionTmuxInfoForTest(actionID, nil, nil, ptr(time.Now().Add(-2*time.Hour))); err != nil {
		t.Fatalf("set started_at: %v", err)
	}

	reader := &fakeBgStateReader{}
	reader.setErr("eeeeeeee", &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist})

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Minute,
	}

	reapBg(cfg, time.Now())

	updated, _ := d.GetAction(actionID)
	if updated.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q after grace period", updated.Status, db.ActionStatusFailed)
	}
	if !strings.Contains(updated.Result.String, "daemon job dir missing") {
		t.Errorf("failure reason missing: %q", updated.Result.String)
	}
}

func TestReapBg_MissingJobWithinGraceIsKept(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive","daemon_short":"ffffffff"}`)
	claimRunning(t, d, actionID)

	reader := &fakeBgStateReader{}
	reader.setErr("ffffffff", &fs.PathError{Op: "open", Path: "x", Err: fs.ErrNotExist})

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Hour,
	}

	reapBg(cfg, time.Now())

	updated, _ := d.GetAction(actionID)
	if updated.Status != db.ActionStatusRunning {
		t.Errorf("status = %q, want %q (within grace)", updated.Status, db.ActionStatusRunning)
	}
}

func TestReapOrphans_MarksRunningWithoutDaemonShortAsFailed(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive"}`)
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
	if updated.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q (orphan reaped)", updated.Status, db.ActionStatusFailed)
	}
	if !strings.Contains(updated.Result.String, "orphaned") {
		t.Errorf("reason = %q, want substring %q", updated.Result.String, "orphaned")
	}
}

func TestReapOrphans_SkipsCloudExecutor(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive","executor":"cloud"}`)
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
	if updated.Status != db.ActionStatusRunning {
		t.Errorf("cloud executor orphan unexpectedly reaped: status = %q", updated.Status)
	}
}

func TestReapBg_WedgedWorkingHardTimeout(t *testing.T) {
	d, actionID := setupTaskAndAction(t, `{"instruction":"x","mode":"interactive","daemon_short":"abcd1234"}`)
	claimRunning(t, d, actionID)
	if err := d.SetActionTmuxInfoForTest(actionID, nil, nil, ptr(time.Now().Add(-5*time.Hour))); err != nil {
		t.Fatalf("set started_at: %v", err)
	}

	reader := &fakeBgStateReader{}
	reader.set("abcd1234", []byte(`{"state":"working"}`))

	cfg := WorkerConfig{
		DispatchConfig:    DispatchConfig{DB: d},
		BgStateReader:     reader,
		BgMissingJobGrace: time.Minute,
		BgHardTimeout:     4 * time.Hour,
	}

	reapBg(cfg, time.Now())

	updated, _ := d.GetAction(actionID)
	if updated.Status != db.ActionStatusFailed {
		t.Errorf("status = %q, want %q (hard timeout)", updated.Status, db.ActionStatusFailed)
	}
	if !strings.Contains(updated.Result.String, "hard timeout") {
		t.Errorf("reason = %q, want substring %q", updated.Result.String, "hard timeout")
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
