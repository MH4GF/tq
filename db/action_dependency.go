package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	DepTypeAction = "action"
	DepTypeTask   = "task"
)

// ActionDep is one dependency edge: the owning action is blocked until the
// referenced action/task reaches a successful terminal state.
type ActionDep struct {
	Type string
	ID   int64
}

// ActionDepStatus is ActionDep enriched with the blocker's current state, used
// by list/get/TUI/triage to explain why an action is still pending.
type ActionDepStatus struct {
	Type          string
	ID            int64
	Satisfied     bool
	BlockerStatus string
}

// dependencyGatePredicate excludes any action (aliased `a`) that still has an
// unsatisfied dependency: a blocker is satisfied only when it reaches a
// successful terminal state (action=done / task=done|archived). A blocker that
// ends failed/cancelled keeps the action pending forever (by design — rescue
// via the tq:dep-triage skill). Keep NextPending and CountPendingByDispatch
// using this so the TUI pending count matches what is actually dispatchable.
//
// The satisfied-state definition is duplicated here (SQL) and in depSatisfied
// (Go, used for display); the two MUST stay in lockstep — change both together.
//
// Perf: the NOT EXISTS correlation is covered by idx_action_deps_action, and
// the inner lookups are by primary key, so a no-dependency action (the common
// case) costs one empty index probe. The known cost is that NextPending's
// `ORDER BY a.id ASC LIMIT 1` can no longer stop at the first time-ready row —
// a low-id prefix of blocked-forever actions is walked every poll until
// triaged away (each step still index-backed; operationally bounded by the
// tq:dep-triage skill).
const dependencyGatePredicate = `
	NOT EXISTS (
	  SELECT 1 FROM action_dependencies dep
	  WHERE dep.action_id = a.id
	    AND NOT (
	      (dep.dep_type = 'action' AND EXISTS (
	         SELECT 1 FROM actions adone WHERE adone.id = dep.dep_id AND adone.status = 'done'))
	   OR (dep.dep_type = 'task' AND EXISTS (
	         SELECT 1 FROM tasks tdone WHERE tdone.id = dep.dep_id AND tdone.status IN ('done','archived')))
	    )
	)`

func depSatisfied(depType, blockerStatus string) bool {
	switch depType {
	case DepTypeAction:
		return blockerStatus == ActionStatusDone
	case DepTypeTask:
		return blockerStatus == TaskStatusDone || blockerStatus == TaskStatusArchived
	default:
		return false
	}
}

// AddActionDependencies inserts dependency edges for actionID. Each dep is
// validated to exist; self-reference and action→action cycles are rejected.
// Duplicate edges are ignored (PK). The whole batch is tx-atomic.
func (db *DB) AddActionDependencies(actionID int64, deps []ActionDep) error {
	if len(deps) == 0 {
		return nil
	}
	ctx := context.Background()
	err := db.withTxRetry(ctx, "AddActionDependencies", func(tx *sql.Tx) error {
		return insertActionDependenciesTx(ctx, tx, actionID, deps, true)
	})
	if err != nil {
		return err
	}
	db.emitEvent("action", actionID, "action.dependencies_added", map[string]any{
		"count": len(deps),
	})
	return nil
}

func insertActionDependenciesTx(ctx context.Context, tx *sql.Tx, actionID int64, deps []ActionDep, checkActionExists bool) error {
	if len(deps) == 0 {
		return nil
	}

	var exists int
	if checkActionExists {
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM actions WHERE id = ?", actionID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("action #%d not found", actionID)
			}
			return err
		}
	}

	for _, d := range deps {
		switch d.Type {
		case DepTypeAction:
			if d.ID == actionID {
				return fmt.Errorf("action #%d cannot depend on itself", actionID)
			}
			if err := tx.QueryRowContext(ctx, "SELECT 1 FROM actions WHERE id = ?", d.ID).Scan(&exists); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("blocked-by action #%d not found", d.ID)
				}
				return err
			}
			cyclic, err := actionReachable(ctx, tx, d.ID, actionID)
			if err != nil {
				return err
			}
			if cyclic {
				return fmt.Errorf("circular dependency: action #%d already (transitively) depends on action #%d", d.ID, actionID)
			}
		case DepTypeTask:
			if err := tx.QueryRowContext(ctx, "SELECT 1 FROM tasks WHERE id = ?", d.ID).Scan(&exists); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("blocked-by task #%d not found", d.ID)
				}
				return err
			}
		default:
			return fmt.Errorf("invalid dependency type %q: must be %q or %q", d.Type, DepTypeAction, DepTypeTask)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT OR IGNORE INTO action_dependencies (action_id, dep_type, dep_id) VALUES (?, ?, ?)",
			actionID, d.Type, d.ID,
		); err != nil {
			return fmt.Errorf("insert dependency action #%d -> %s #%d: %w", actionID, d.Type, d.ID, err)
		}
	}

	return nil
}

// actionReachable reports whether target is reachable from start by following
// action→action dependency edges (start depends-on ... depends-on target).
// Adding edge target→start would then close a cycle.
func actionReachable(ctx context.Context, tx *sql.Tx, start, target int64) (bool, error) {
	var hit int
	err := tx.QueryRowContext(ctx, `
		WITH RECURSIVE reach(id) AS (
		  SELECT ?
		  UNION
		  SELECT d.dep_id FROM action_dependencies d
		  JOIN reach r ON d.action_id = r.id
		  WHERE d.dep_type = 'action'
		)
		SELECT 1 FROM reach WHERE id = ? LIMIT 1`,
		start, target,
	).Scan(&hit)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (db *DB) ReplaceActionDependencies(actionID int64, deps []ActionDep) error {
	ctx := context.Background()
	var clearedCount int64
	err := db.withTxRetry(ctx, "ReplaceActionDependencies", func(tx *sql.Tx) error {
		clearedCount = 0
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT 1 FROM actions WHERE id = ?", actionID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("action #%d not found", actionID)
			}
			return err
		}
		res, err := tx.ExecContext(ctx, "DELETE FROM action_dependencies WHERE action_id = ?", actionID)
		if err != nil {
			return err
		}
		clearedCount, err = res.RowsAffected()
		if err != nil {
			return err
		}
		return insertActionDependenciesTx(ctx, tx, actionID, deps, false)
	})
	if err != nil {
		return fmt.Errorf("replace dependencies for action #%d: %w", actionID, err)
	}
	if clearedCount > 0 {
		db.emitEvent("action", actionID, "action.dependencies_cleared", map[string]any{
			"count": clearedCount,
		})
	}
	if len(deps) > 0 {
		db.emitEvent("action", actionID, "action.dependencies_added", map[string]any{
			"count": len(deps),
		})
	}
	return nil
}

// ClearActionDependencies removes all dependency edges for actionID. Combined
// with AddActionDependencies it implements replace semantics.
func (db *DB) ClearActionDependencies(actionID int64) error {
	var rowsAffected int64
	if err := withRetry(context.Background(), "ClearActionDependencies", func() error {
		res, err := db.Exec("DELETE FROM action_dependencies WHERE action_id = ?", actionID)
		if err != nil {
			return err
		}
		rowsAffected, err = res.RowsAffected()
		return err
	}); err != nil {
		return fmt.Errorf("clear dependencies for action #%d: %w", actionID, err)
	}
	if rowsAffected > 0 {
		db.emitEvent("action", actionID, "action.dependencies_cleared", map[string]any{
			"count": rowsAffected,
		})
	}
	return nil
}

const actionDepStatusSelect = `
	SELECT d.action_id, d.dep_type, d.dep_id,
	       COALESCE(da.status, dt.status, '') AS blocker_status
	FROM action_dependencies d
	LEFT JOIN actions da ON d.dep_type = 'action' AND da.id = d.dep_id
	LEFT JOIN tasks   dt ON d.dep_type = 'task'   AND dt.id = d.dep_id`

// ListActionDependencies returns the dependency edges of actionID enriched with
// each blocker's current status and whether it is satisfied.
func (db *DB) ListActionDependencies(actionID int64) ([]ActionDepStatus, error) {
	byID, err := db.ListActionDependenciesByActionIDs([]int64{actionID})
	if err != nil {
		return nil, err
	}
	return byID[actionID], nil
}

// ListActionDependenciesByActionIDs bulk-loads dependencies for many actions in
// one query (callers iterating actions must avoid N+1 — Rule 15).
func (db *DB) ListActionDependenciesByActionIDs(ids []int64) (map[int64][]ActionDepStatus, error) {
	result := make(map[int64][]ActionDepStatus)
	if len(ids) == 0 {
		return result, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	rows, err := db.Query(actionDepStatusSelect+`
		WHERE d.action_id IN (`+strings.Join(placeholders, ", ")+`)
		ORDER BY d.action_id, d.dep_type, d.dep_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var aid int64
		var s ActionDepStatus
		if err := rows.Scan(&aid, &s.Type, &s.ID, &s.BlockerStatus); err != nil {
			return nil, err
		}
		s.Satisfied = depSatisfied(s.Type, s.BlockerStatus)
		result[aid] = append(result[aid], s)
	}
	return result, rows.Err()
}
