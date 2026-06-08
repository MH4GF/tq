package db

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math/rand/v2"
	"os"
	"strings"
	"time"
)

const (
	retryMaxAttempts = 8
	retryBaseDelay   = 5 * time.Millisecond
	retryCapDelay    = 200 * time.Millisecond
	retryTotalBudget = 3 * time.Second
)

var (
	retryNow   = time.Now
	retrySleep = time.Sleep
	retryRand  = rand.Float64
)

func isRetryableLockError(err error) bool {
	for cur := err; cur != nil; cur = errors.Unwrap(cur) {
		msg := strings.ToUpper(cur.Error())
		if strings.Contains(msg, "DATABASE IS LOCKED") ||
			strings.Contains(msg, "SQLITE_BUSY") ||
			strings.Contains(msg, "SQLITE_LOCKED") {
			return true
		}
	}
	return false
}

func withRetry(ctx context.Context, op string, fn func() error) error {
	start := retryNow()
	for attempt := range retryMaxAttempts {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryableLockError(err) {
			return err
		}
		elapsed := retryNow().Sub(start)
		if elapsed >= retryTotalBudget || attempt+1 >= retryMaxAttempts {
			logRetryGiveUp(op, attempt+1, elapsed, err)
			return err
		}
		delay := backoffDelay(attempt)
		logRetryAttempt(op, attempt+1, delay, err)
		retrySleep(delay)
	}
	return nil
}

func (db *DB) withTxRetry(ctx context.Context, op string, fn func(*sql.Tx) error) error {
	return withRetry(ctx, op, func() error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := fn(tx); err != nil {
			_ = tx.Rollback()
			return err
		}
		return tx.Commit()
	})
}

func backoffDelay(attempt int) time.Duration {
	shift := min(attempt, 8)
	d := min(retryBaseDelay<<shift, retryCapDelay)
	jitter := 0.5 + retryRand()
	return time.Duration(float64(d) * jitter)
}

func retryDebug() bool {
	return os.Getenv("TQ_LOG_RETRY") == "1"
}

func logRetryAttempt(op string, attempt int, delay time.Duration, err error) {
	if !retryDebug() {
		return
	}
	slog.Debug("db retry",
		"op", op,
		"attempt", attempt,
		"delay", delay,
		"err", err.Error(),
	)
}

func logRetryGiveUp(op string, attempts int, elapsed time.Duration, err error) {
	if !retryDebug() {
		return
	}
	slog.Debug("db retry give up",
		"op", op,
		"attempts", attempts,
		"elapsed", elapsed,
		"err", err.Error(),
	)
}
