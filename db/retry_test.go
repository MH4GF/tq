package db

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestIsRetryableLockError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated", errors.New("syntax error"), false},
		{"modernc locked", errors.New("SQLite error: database is locked"), true},
		{"libsql busy", errors.New("SQLITE_BUSY: database is locked"), true},
		{"locked code only", errors.New("SQLITE_LOCKED: lock conflict"), true},
		{"upper case message", errors.New("ERROR: DATABASE IS LOCKED"), true},
		{"wrapped", fmt.Errorf("failed to execute SQL: %w", errors.New("database is locked")), true},
		{"deeply wrapped", fmt.Errorf("a: %w", fmt.Errorf("b: %w", errors.New("SQLite error: database is locked"))), true},
		{"sql.ErrNoRows", errors.New("sql: no rows in result set"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableLockError(tc.err); got != tc.want {
				t.Fatalf("isRetryableLockError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

type retryFixture struct {
	t        *testing.T
	now      time.Time
	sleeps   []time.Duration
	origNow  func() time.Time
	origSlp  func(time.Duration)
	origRand func() float64
}

func newRetryFixture(t *testing.T) *retryFixture {
	t.Helper()
	f := &retryFixture{
		t:        t,
		now:      time.Unix(0, 0),
		origNow:  retryNow,
		origSlp:  retrySleep,
		origRand: retryRand,
	}
	retryNow = func() time.Time { return f.now }
	retrySleep = func(d time.Duration) {
		f.sleeps = append(f.sleeps, d)
		f.now = f.now.Add(d)
	}
	retryRand = func() float64 { return 0.5 }
	t.Cleanup(func() {
		retryNow = f.origNow
		retrySleep = f.origSlp
		retryRand = f.origRand
	})
	return f
}

func TestWithRetry_SucceedsAfterRetries(t *testing.T) {
	f := newRetryFixture(t)
	calls := 0
	err := withRetry(context.Background(), "op", func() error {
		calls++
		if calls < 4 {
			return errors.New("SQLite error: database is locked")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withRetry returned err: %v", err)
	}
	if calls != 4 {
		t.Fatalf("expected 4 calls, got %d", calls)
	}
	if len(f.sleeps) != 3 {
		t.Fatalf("expected 3 sleeps, got %d", len(f.sleeps))
	}
}

func TestWithRetry_NonRetryableReturnsImmediately(t *testing.T) {
	f := newRetryFixture(t)
	calls := 0
	sentinel := errors.New("syntax error")
	err := withRetry(context.Background(), "op", func() error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
	if len(f.sleeps) != 0 {
		t.Fatalf("expected 0 sleeps, got %d", len(f.sleeps))
	}
}

func TestWithRetry_BudgetExhausted(t *testing.T) {
	newRetryFixture(t)
	calls := 0
	lastErr := errors.New("SQLite error: database is locked")
	err := withRetry(context.Background(), "op", func() error {
		calls++
		return lastErr
	})
	if !errors.Is(err, lastErr) {
		t.Fatalf("expected last err to surface raw, got %v", err)
	}
	if calls != retryMaxAttempts {
		t.Fatalf("expected %d calls, got %d", retryMaxAttempts, calls)
	}
}

func TestWithRetry_CtxCancelled(t *testing.T) {
	newRetryFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	err := withRetry(ctx, "op", func() error {
		calls++
		cancel()
		return errors.New("SQLite error: database is locked")
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call before cancel, got %d", calls)
	}
}

func TestBackoffDelay_BoundedAndJittered(t *testing.T) {
	newRetryFixture(t)
	for attempt := 0; attempt < 16; attempt++ {
		d := backoffDelay(attempt)
		if d < 0 {
			t.Fatalf("attempt %d: negative delay %v", attempt, d)
		}
		max := time.Duration(float64(retryCapDelay) * 1.5)
		if d > max {
			t.Fatalf("attempt %d: delay %v exceeds cap*1.5 (%v)", attempt, d, max)
		}
	}
}
