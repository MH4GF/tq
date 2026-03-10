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
  url         TEXT,
  metadata    TEXT NOT NULL DEFAULT '{}',
  status      TEXT DEFAULT 'open',
  work_dir    TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at  TEXT
);

CREATE TABLE IF NOT EXISTS actions (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  title        TEXT NOT NULL DEFAULT '',
  prompt_id    TEXT NOT NULL,
  task_id      INTEGER REFERENCES tasks(id),
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
