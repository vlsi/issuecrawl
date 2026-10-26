-- Subset of the gadak schema (github.com/midagedev/gadak), copied from a live gadak.db
-- with triggers and indexes left out. issues_full is gadak's documented contract.
CREATE TABLE sources (
  id        TEXT PRIMARY KEY,
  kind      TEXT NOT NULL,
  base_url  TEXT,
  synced_at TEXT
);
CREATE TABLE items (
  id          TEXT PRIMARY KEY,
  source_id   TEXT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
  kind        TEXT NOT NULL,
  external_id TEXT,
  key         TEXT,
  title       TEXT,
  body_text   TEXT,
  author      TEXT,
  author_id   TEXT,
  url         TEXT,
  created_at  TEXT,
  updated_at  TEXT,
  synced_at   TEXT
);
CREATE TABLE "issues_raw" (
  item_id             TEXT PRIMARY KEY REFERENCES items(id) ON DELETE CASCADE,
  key                 TEXT NOT NULL,
  project_key         TEXT,
  issue_type          TEXT,
  issue_type_id       TEXT,
  status              TEXT,
  status_id           TEXT,
  status_category     TEXT,
  priority            TEXT,
  priority_rank       INTEGER NOT NULL DEFAULT 0,
  assignee            TEXT,
  assignee_id         TEXT,
  assignee_email      TEXT,
  reporter            TEXT,
  reporter_id         TEXT,
  reporter_email      TEXT,
  parent_key          TEXT,
  labels              TEXT,
  components          TEXT,
  fix_versions        TEXT,
  affects_versions    TEXT,
  environment_text    TEXT,
  duedate             TEXT,
  resolution          TEXT,
  created_at          TEXT,
  updated_at          TEXT,
  status_changed_at   TEXT,
  resolved_at         TEXT,
  reopen_count        INTEGER NOT NULL DEFAULT 0,
  reopened_at         TEXT,
  assignee_changed_at TEXT,
  comment_count       INTEGER NOT NULL DEFAULT 0,
  description_adf     TEXT,
  custom              TEXT,
  raw                 TEXT
, reopen_reason TEXT NOT NULL DEFAULT '', cloned_from TEXT NOT NULL DEFAULT '', hierarchy_level INTEGER NOT NULL DEFAULT 0, epic_key TEXT, priority_id TEXT NOT NULL DEFAULT '', resolution_id TEXT NOT NULL DEFAULT '', sprint_id INTEGER, sprint_name TEXT, sprint_state TEXT, fix_version_ids TEXT, security_level_id TEXT, security_level TEXT, started_at TEXT, cycle_hours REAL, last_activity_at TEXT, open_blockers INTEGER NOT NULL DEFAULT 0, carryover_count INTEGER, first_sprint_id INTEGER, first_sprint_at TEXT, blocked_hours REAL, blocked_since TEXT);
CREATE TABLE "comments" (
  id          TEXT NOT NULL,
  item_id     TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  external_id TEXT,
  author      TEXT,
  author_id   TEXT,
  body_adf    TEXT,
  body_text   TEXT,
  created_at  TEXT,
  updated_at  TEXT,
  visibility_type  TEXT NOT NULL DEFAULT '',
  visibility_value TEXT NOT NULL DEFAULT '',
  jsd_public  INTEGER, parent_id TEXT,
  PRIMARY KEY (item_id, id)
);
CREATE TABLE links (
  item_id    TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
  type       TEXT NOT NULL,
  direction  TEXT NOT NULL,
  target_key TEXT NOT NULL,
  PRIMARY KEY (item_id, type, direction, target_key)
);
CREATE VIEW issues AS
  SELECT it.title AS summary, i.*, COALESCE(it.body_text, '') AS description_text
  FROM issues_raw i JOIN items it ON it.id = i.item_id;
CREATE VIEW issues_full AS SELECT * FROM issues;
