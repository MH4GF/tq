package db

import (
	"database/sql"
	"fmt"
	"time"
)

func (db *DB) UpdateWorkerHeartbeat() error {
	_, err := db.Exec("REPLACE INTO worker_heartbeats(id, last_heartbeat) VALUES(1, datetime('now'))")
	if err != nil {
		return fmt.Errorf("update worker heartbeat: %w", err)
	}
	return nil
}

func (db *DB) IsWorkerRunning(staleThreshold time.Duration) (bool, error) {
	var heartbeat string
	err := db.QueryRow("SELECT last_heartbeat FROM worker_heartbeats WHERE id = 1").Scan(&heartbeat)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query worker heartbeat: %w", err)
	}
	t, err := time.Parse(TimeLayout, heartbeat)
	if err != nil {
		return false, fmt.Errorf("parse worker heartbeat: %w", err)
	}
	return time.Since(t) < staleThreshold, nil
}
