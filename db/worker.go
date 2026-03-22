package db

import (
	"database/sql"
	"fmt"
	"time"
)

type workerHeartbeat struct {
	MaxInteractive int
}

func (db *DB) getActiveWorker(staleThreshold time.Duration) (*workerHeartbeat, error) {
	var heartbeat string
	var maxInteractive int
	err := db.QueryRow("SELECT last_heartbeat, max_interactive FROM worker_heartbeats WHERE id = 1").Scan(&heartbeat, &maxInteractive)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, fmt.Errorf("query worker heartbeat: %w", err)
	}
	t, err := time.Parse(TimeLayout, heartbeat)
	if err != nil {
		return nil, fmt.Errorf("parse worker heartbeat: %w", err)
	}
	if time.Since(t) >= staleThreshold {
		return nil, sql.ErrNoRows
	}
	return &workerHeartbeat{MaxInteractive: maxInteractive}, nil
}

func (db *DB) UpdateWorkerHeartbeat(maxInteractive int) error {
	_, err := db.Exec("REPLACE INTO worker_heartbeats(id, last_heartbeat, max_interactive) VALUES(1, datetime('now'), ?)", maxInteractive)
	if err != nil {
		return fmt.Errorf("update worker heartbeat: %w", err)
	}
	return nil
}

func (db *DB) GetWorkerMaxInteractive(staleThreshold time.Duration) (int, error) {
	w, err := db.getActiveWorker(staleThreshold)
	if err != nil {
		return 0, err
	}
	return w.MaxInteractive, nil
}

func (db *DB) IsWorkerRunning(staleThreshold time.Duration) (bool, error) {
	_, err := db.getActiveWorker(staleThreshold)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
