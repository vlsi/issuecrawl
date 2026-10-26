-- Subset of the gitcrawl schema (github.com/openclaw/gitcrawl, internal/store/schema.go),
-- copied from a live gitcrawl.db with triggers and indexes left out.
CREATE TABLE repositories (
  id integer primary key,
  owner text not null,
  name text not null,
  full_name text not null unique,
  github_repo_id text,
  raw_json text not null,
  updated_at text not null
);
CREATE TABLE threads (
  id integer primary key,
  repo_id integer not null references repositories(id) on delete cascade,
  github_id text not null,
  number integer not null,
  kind text not null check (kind in ('issue', 'pull_request')),
  state text not null,
  title text not null,
  body text,
  author_login text,
  author_type text,
  author_association text,
  html_url text not null,
  labels_json text not null,
  assignees_json text not null,
  raw_json text not null,
  content_hash text not null,
  is_draft integer not null default 0,
  created_at_gh text,
  updated_at_gh text,
  closed_at_gh text,
  merged_at_gh text,
  closed_at_local text,
  close_reason_local text,
  first_pulled_at text,
  last_pulled_at text,
  observation_sequence integer not null default 0
    check (
      typeof(observation_sequence) = 'integer'
      and observation_sequence >= -9223372036854775807
    ),
  evidence_observation_sequence integer not null default 0
    check (typeof(evidence_observation_sequence) = 'integer' and evidence_observation_sequence >= 0),
  evidence_source_updated_at text not null default '',
  updated_at text not null,
  unique(repo_id, kind, number)
);
CREATE TABLE comments (
  id integer primary key,
  thread_id integer not null references threads(id) on delete cascade,
  github_id text not null,
  comment_type text not null,
  author_login text,
  author_type text,
  body text not null,
  is_bot integer not null default 0,
  raw_json text not null,
  raw_json_blob_id integer references blobs(id) on delete set null,
  created_at_gh text,
  updated_at_gh text,
  deleted_at text,
  deletion_reason text,
  check (
    (deleted_at is null and deletion_reason is null)
    or (deleted_at is not null and deletion_reason is not null and trim(deletion_reason) <> '')
  ),
  unique(thread_id, comment_type, github_id)
);
CREATE TABLE pull_request_details (
  thread_id integer primary key references threads(id) on delete cascade,
  repo_id integer not null references repositories(id) on delete cascade,
  number integer not null,
  base_sha text,
  head_sha text,
  head_ref text,
  head_repo_full_name text,
  mergeable_state text,
  additions integer not null default 0,
  deletions integer not null default 0,
  changed_files integer not null default 0,
  raw_json text not null,
  fetched_at text not null,
  updated_at text not null,
  unique(repo_id, number)
);
CREATE TABLE pull_request_commits (
  thread_id integer not null references threads(id) on delete cascade,
  sha text not null,
  message text,
  author_login text,
  author_name text,
  committed_at text,
  html_url text,
  raw_json text not null,
  fetched_at text not null,
  deleted_at text,
  deletion_reason text,
  check (
    (deleted_at is null and deletion_reason is null)
    or (deleted_at is not null and deletion_reason is not null and trim(deletion_reason) <> '')
  ),
  primary key(thread_id, sha)
);
CREATE TABLE sync_runs (
  id integer primary key,
  repo_id integer references repositories(id) on delete cascade,
  scope text not null,
  status text not null,
  started_at text not null,
  finished_at text,
  stats_json text,
  error_text text
);
