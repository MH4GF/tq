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
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  title           TEXT NOT NULL DEFAULT '',
  task_id         INTEGER NOT NULL REFERENCES tasks(id),
  metadata        TEXT NOT NULL DEFAULT '{}',
  status          TEXT DEFAULT 'pending',
  result          TEXT,
  tmux_session    TEXT,
  tmux_window     TEXT,
  dispatch_after  TEXT,
  work_dir        TEXT NOT NULL DEFAULT '',
  created_at      TEXT NOT NULL DEFAULT (datetime('now')),
  started_at      TEXT,
  completed_at    TEXT
);

CREATE INDEX IF NOT EXISTS idx_actions_dispatch ON actions(status, id ASC);
CREATE INDEX IF NOT EXISTS idx_actions_task ON actions(task_id, id ASC);
CREATE INDEX IF NOT EXISTS idx_tasks_project ON tasks(project_id, id DESC);

CREATE TABLE IF NOT EXISTS action_dependencies (
  action_id  INTEGER NOT NULL REFERENCES actions(id) ON DELETE CASCADE,
  dep_type   TEXT NOT NULL CHECK (dep_type IN ('action','task')),
  dep_id     INTEGER NOT NULL,
  PRIMARY KEY (action_id, dep_type, dep_id)
);

CREATE INDEX IF NOT EXISTS idx_action_deps_action ON action_dependencies(action_id);
CREATE INDEX IF NOT EXISTS idx_action_deps_dep ON action_dependencies(dep_type, dep_id);

-- action_dependencies.dep_id is polymorphic (dep_type action|task) with no FK,
-- so deleting the referenced task/action leaves dangling edges that the
-- dependency gate can never satisfy. Purge them on delete (all delete paths).
CREATE TRIGGER IF NOT EXISTS trg_action_deps_purge_task
AFTER DELETE ON tasks
BEGIN
  DELETE FROM action_dependencies
  WHERE dep_type = 'task' AND dep_id = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_action_deps_purge_action
AFTER DELETE ON actions
BEGIN
  DELETE FROM action_dependencies
  WHERE dep_type = 'action' AND dep_id = OLD.id;
END;

CREATE TABLE IF NOT EXISTS task_action_counts (
  task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  status  TEXT NOT NULL,
  count   INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (task_id, status)
);

CREATE TRIGGER IF NOT EXISTS trg_actions_count_insert
AFTER INSERT ON actions
BEGIN
  INSERT INTO task_action_counts (task_id, status, count)
  VALUES (NEW.task_id, NEW.status, 1)
  ON CONFLICT(task_id, status) DO UPDATE SET count = count + 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_actions_count_update
AFTER UPDATE OF status ON actions
WHEN OLD.status != NEW.status
BEGIN
  UPDATE task_action_counts SET count = count - 1
  WHERE task_id = OLD.task_id AND status = OLD.status;
  INSERT INTO task_action_counts (task_id, status, count)
  VALUES (NEW.task_id, NEW.status, 1)
  ON CONFLICT(task_id, status) DO UPDATE SET count = count + 1;
END;

CREATE TRIGGER IF NOT EXISTS trg_actions_count_delete
AFTER DELETE ON actions
BEGIN
  UPDATE task_action_counts SET count = count - 1
  WHERE task_id = OLD.task_id AND status = OLD.status;
END;

CREATE TABLE IF NOT EXISTS schedules (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id     INTEGER NOT NULL REFERENCES tasks(id),
  instruction TEXT NOT NULL,
  title       TEXT NOT NULL,
  cron_expr   TEXT NOT NULL,
  metadata    TEXT NOT NULL DEFAULT '{}',
  enabled     INTEGER NOT NULL DEFAULT 1,
  last_run_at TEXT,
  last_error  TEXT,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_schedules_task ON schedules(task_id, enabled);

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
  id              INTEGER PRIMARY KEY CHECK (id = 1),
  last_heartbeat  TEXT NOT NULL,
  max_interactive INTEGER NOT NULL DEFAULT 3
);

-- Global key-value settings (e.g. default dispatch mode). Travels with the DB
-- so the configuration follows libsql/Turso endpoints, not a local file.
CREATE TABLE IF NOT EXISTS settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- FTS5 search indexes (trigram tokenizer: language-agnostic substring match,
-- preserves the old LIKE '%kw%' behavior for >=3-character keywords incl. CJK).
-- rowid mirrors the source row id so Search can join back for the other columns.
-- Backfill of pre-existing rows is done idempotently in db.go Migrate().
CREATE VIRTUAL TABLE IF NOT EXISTS tasks_fts USING fts5(
  title, metadata, tokenize='trigram'
);

CREATE VIRTUAL TABLE IF NOT EXISTS actions_fts USING fts5(
  title, result, metadata, tokenize='trigram'
);

CREATE VIRTUAL TABLE IF NOT EXISTS events_fts USING fts5(
  reason, tokenize='trigram'
);

CREATE TRIGGER IF NOT EXISTS trg_tasks_fts_insert
AFTER INSERT ON tasks
BEGIN
  INSERT INTO tasks_fts (rowid, title, metadata)
  VALUES (NEW.id, NEW.title, NEW.metadata);
END;

CREATE TRIGGER IF NOT EXISTS trg_tasks_fts_update
AFTER UPDATE OF title, metadata ON tasks
BEGIN
  DELETE FROM tasks_fts WHERE rowid = OLD.id;
  INSERT INTO tasks_fts (rowid, title, metadata)
  VALUES (NEW.id, NEW.title, NEW.metadata);
END;

CREATE TRIGGER IF NOT EXISTS trg_tasks_fts_delete
AFTER DELETE ON tasks
BEGIN
  DELETE FROM tasks_fts WHERE rowid = OLD.id;
END;

CREATE TRIGGER IF NOT EXISTS trg_actions_fts_insert
AFTER INSERT ON actions
BEGIN
  INSERT INTO actions_fts (rowid, title, result, metadata)
  VALUES (NEW.id, NEW.title, COALESCE(NEW.result, ''), NEW.metadata);
END;

CREATE TRIGGER IF NOT EXISTS trg_actions_fts_update
AFTER UPDATE OF title, result, metadata ON actions
BEGIN
  DELETE FROM actions_fts WHERE rowid = OLD.id;
  INSERT INTO actions_fts (rowid, title, result, metadata)
  VALUES (NEW.id, NEW.title, COALESCE(NEW.result, ''), NEW.metadata);
END;

CREATE TRIGGER IF NOT EXISTS trg_actions_fts_delete
AFTER DELETE ON actions
BEGIN
  DELETE FROM actions_fts WHERE rowid = OLD.id;
END;

-- events is append-only (event.go only ever INSERTs). Index only the
-- task.status_changed reason that db.Search exposes; skip empty reasons so an
-- empty-reason status change is not searchable (matches prior LIKE behavior).
CREATE TRIGGER IF NOT EXISTS trg_events_fts_insert
AFTER INSERT ON events
WHEN NEW.entity_type = 'task'
  AND NEW.event_type = 'task.status_changed'
  AND json_extract(NEW.payload, '$.reason') IS NOT NULL
  AND json_extract(NEW.payload, '$.reason') != ''
BEGIN
  INSERT INTO events_fts (rowid, reason)
  VALUES (NEW.id, json_extract(NEW.payload, '$.reason'));
END;
