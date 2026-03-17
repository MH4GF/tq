package db

import (
	"database/sql"
	"encoding/json"
	"log/slog"
)

type Event struct {
	ID         int64  `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	EventType  string `json:"event_type"`
	Payload    string `json:"payload"`
	CreatedAt  string `json:"created_at"`
}

const eventColumns = "id, entity_type, entity_id, event_type, payload, created_at"

func (e *Event) scanFields() []any {
	return []any{&e.ID, &e.EntityType, &e.EntityID, &e.EventType, &e.Payload, &e.CreatedAt}
}

func scanEvents(rows *sql.Rows) ([]Event, error) {
	defer func() { _ = rows.Close() }()
	var events []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(e.scanFields()...); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (db *DB) emitEvent(entityType string, entityID int64, eventType string, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Warn("event: marshal payload", "error", err, "event_type", eventType)
		return
	}
	_, err = db.Exec(
		"INSERT INTO events (entity_type, entity_id, event_type, payload) VALUES (?, ?, ?, ?)",
		entityType, entityID, eventType, string(data),
	)
	if err != nil {
		slog.Warn("event: insert", "error", err, "event_type", eventType)
	}
}

func (db *DB) ListEvents(entityType string, entityID int64) ([]Event, error) {
	rows, err := db.Query(
		"SELECT "+eventColumns+" FROM events WHERE entity_type = ? AND entity_id = ? ORDER BY id",
		entityType, entityID,
	)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}

func (db *DB) ListRecentEvents(limit int) ([]Event, error) {
	rows, err := db.Query(
		"SELECT "+eventColumns+" FROM events ORDER BY id DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	return scanEvents(rows)
}
