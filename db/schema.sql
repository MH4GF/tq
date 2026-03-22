CREATE TABLE IF NOT EXISTS projects (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  name              TEXT NOT NULL UNIQUE,
  work_dir          TEXT NOT NULL,
  metadata          TEXT NOT NULL DEFAULT '{}',
  dispatch_enabled  INTEGER NOT NULL DEFAULT 1,
  created_at        TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  project_id  INTEGER NOT NULL REFERENCES projects(id),
  title       TEXT NOT NULL,
  metadata    TEXT NOT NULL DEFAULT '{}',
  status      TEXT DEFAULT 'open',
  work_dir    TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at  TEXT
);

CREATE TABLE IF NOT EXISTS actions (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL DEFAULT '',
  prompt_id    TEXT DEFAULT '',
  task_id      INTEGER NOT NULL REFERENCES tasks(id),
  metadata     TEXT NOT NULL DEFAULT '{}',
  status       TEXT DEFAULT 'pending',
  result       TEXT,
  session_id   TEXT,
  tmux_pane    TEXT,
  created_at   TEXT NOT NULL DEFAULT (datetime('now')),
  started_at   TEXT,
  completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_actions_dispatch ON actions(status, id ASC);
CREATE INDEX IF NOT EXISTS idx_actions_task ON actions(task_id, id ASC);

CREATE TABLE IF NOT EXISTS schedules (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id     INTEGER NOT NULL REFERENCES tasks(id),
  prompt_id   TEXT NOT NULL DEFAULT '',
  title       TEXT NOT NULL,
  cron_expr   TEXT NOT NULL,
  metadata    TEXT NOT NULL DEFAULT '{}',
  enabled     INTEGER NOT NULL DEFAULT 1,
  last_run_at TEXT,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS events (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL,
  entity_id   INTEGER NOT NULL,
  event_type  TEXT NOT NULL,
  payload     TEXT NOT NULL DEFAULT '{}',
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_events_entity ON events(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);

CREATE TABLE IF NOT EXISTS worker_heartbeats (
  id             INTEGER PRIMARY KEY CHECK (id = 1),
  last_heartbeat TEXT NOT NULL
);
