package db

import (
	"database/sql"
	"errors"
	"fmt"
)

// SettingDefaultMode is the settings key holding the global default dispatch
// mode applied to new actions when --meta does not specify one.
const SettingDefaultMode = "default_mode"

// GetSetting returns the value for key, or ("", nil) when the key is unset.
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query setting %q: %w", key, err)
	}
	return value, nil
}

// ListSettings returns all stored settings as a key→value map.
func (db *DB) ListSettings() (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("query settings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return out, nil
}

// SetSetting upserts the value for key.
func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(
		"REPLACE INTO settings(key, value, updated_at) VALUES(?, ?, datetime('now'))",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set setting %q: %w", key, err)
	}
	return nil
}
