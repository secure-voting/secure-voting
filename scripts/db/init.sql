-- scripts/db/init.sql
-- Initial PostgreSQL schema for secure-voting (approved DB structure).
-- Runs only on first init of the postgres data volume.

BEGIN;

-- UUID generation
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 1) users
CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  role text NOT NULL CHECK (role IN ('admin', 'voter', 'researcher')),
  full_name text NULL,
  phone text NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- 2) api_tokens
CREATE TABLE IF NOT EXISTS api_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash text NOT NULL,
  scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id ON api_tokens(user_id);

-- 3) audit_log
CREATE TABLE IF NOT EXISTS audit_log (
  id bigserial PRIMARY KEY,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  actor_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
  event_type text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_audit_log_occurred_at ON audit_log(occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_event_type ON audit_log(event_type);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor_user_id ON audit_log(actor_user_id);

-- 4) elections
CREATE TABLE IF NOT EXISTS elections (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  title text NOT NULL,
  description text NULL,

  start_at timestamptz NOT NULL,
  end_at timestamptz NOT NULL,

  tally_rule text NOT NULL,
  ballot_format text NOT NULL CHECK (ballot_format IN ('approval', 'ranking', 'score')),

  committee_size int NULL CHECK (committee_size IS NULL OR committee_size > 0),
  quota_type text NULL CHECK (quota_type IS NULL OR quota_type IN ('hare', 'droop')),

  status text NOT NULL CHECK (status IN ('draft', 'scheduled', 'active', 'paused', 'closed', 'results_ready', 'published')),
  access_mode text NOT NULL CHECK (access_mode IN ('open', 'invite')),

  publish_at timestamptz NULL,
  published_at timestamptz NULL,

  show_aggregates boolean NOT NULL DEFAULT false,

  approval_max_choices int NULL CHECK (approval_max_choices IS NULL OR approval_max_choices > 0),
  ranking_top_k int NULL CHECK (ranking_top_k IS NULL OR ranking_top_k > 0),

  score_min int NULL,
  score_max int NULL,
  score_step int NULL,
  score_allow_skip boolean NOT NULL DEFAULT false,

  created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT chk_elections_time CHECK (start_at < end_at),

  CONSTRAINT chk_elections_approval_fields CHECK (
    ballot_format <> 'approval'
    OR (ranking_top_k IS NULL AND score_min IS NULL AND score_max IS NULL AND score_step IS NULL)
  ),

  CONSTRAINT chk_elections_ranking_fields CHECK (
    ballot_format <> 'ranking'
    OR (approval_max_choices IS NULL AND score_min IS NULL AND score_max IS NULL AND score_step IS NULL)
  ),

  CONSTRAINT chk_elections_score_fields CHECK (
    ballot_format <> 'score'
    OR (
      approval_max_choices IS NULL AND ranking_top_k IS NULL
      AND score_min IS NOT NULL AND score_max IS NOT NULL AND score_step IS NOT NULL
      AND score_step > 0
      AND score_min <= score_max
      AND ((score_max - score_min) % score_step = 0)
    )
  )
);

CREATE INDEX IF NOT EXISTS idx_elections_created_by ON elections(created_by);
CREATE INDEX IF NOT EXISTS idx_elections_status ON elections(status);
CREATE INDEX IF NOT EXISTS idx_elections_access_mode ON elections(access_mode);

-- 5) candidates
CREATE TABLE IF NOT EXISTS candidates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  election_id uuid NOT NULL REFERENCES elections(id) ON DELETE CASCADE,
  name text NOT NULL,
  meta jsonb NULL,
  CONSTRAINT uq_candidates_election_name UNIQUE (election_id, name)
);

CREATE INDEX IF NOT EXISTS idx_candidates_election_id ON candidates(election_id);

-- 6) ballots
CREATE TABLE IF NOT EXISTS ballots (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  election_id uuid NOT NULL REFERENCES elections(id) ON DELETE CASCADE,

  voter_hash text NOT NULL,
  format text NOT NULL CHECK (format IN ('approval', 'ranking', 'score')),

  approval_set jsonb NULL,
  ranking jsonb NULL,
  scores jsonb NULL,

  submitted_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NULL,

  status text NOT NULL CHECK (status IN ('draft', 'accepted', 'rejected')),

  CONSTRAINT uq_ballots_election_voter UNIQUE (election_id, voter_hash),

  CONSTRAINT chk_ballots_format_payload CHECK (
    (format = 'approval' AND ranking IS NULL AND scores IS NULL)
    OR (format = 'ranking' AND approval_set IS NULL AND scores IS NULL)
    OR (format = 'score' AND approval_set IS NULL AND ranking IS NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_ballots_election_id ON ballots(election_id);

-- 7) results
CREATE TABLE IF NOT EXISTS results (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  election_id uuid NOT NULL REFERENCES elections(id) ON DELETE CASCADE,
  version int NOT NULL CHECK (version > 0),
  method text NOT NULL,
  params jsonb NULL,
  winners jsonb NOT NULL,
  metrics jsonb NULL,
  protocol jsonb NULL,
  published_at timestamptz NULL,

  CONSTRAINT uq_results_election_version UNIQUE (election_id, version),
  CONSTRAINT chk_results_winners_is_array CHECK (jsonb_typeof(winners) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_results_election_id ON results(election_id);

-- 8) election_invites
CREATE TABLE IF NOT EXISTS election_invites (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  election_id uuid NOT NULL REFERENCES elections(id) ON DELETE CASCADE,
  email text NOT NULL,
  invite_code_hash text NOT NULL,
  status text NOT NULL CHECK (status IN ('created', 'sent', 'accepted', 'revoked', 'failed')),
  sent_at timestamptz NULL,
  accepted_at timestamptz NULL,
  created_at timestamptz NOT NULL DEFAULT now(),

  CONSTRAINT uq_invites_election_email UNIQUE (election_id, email)
);

CREATE INDEX IF NOT EXISTS idx_invites_election_id ON election_invites(election_id);

-- 9) experiments
CREATE TABLE IF NOT EXISTS experiments (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  type text NOT NULL CHECK (type IN ('algo', 'behavior')),
  params jsonb NOT NULL,
  created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  status text NOT NULL CHECK (status IN ('draft', 'running', 'done', 'error')),
  seed bigint NULL
);

CREATE INDEX IF NOT EXISTS idx_experiments_created_by ON experiments(created_by);
CREATE INDEX IF NOT EXISTS idx_experiments_status ON experiments(status);

-- 10) experiment_runs
CREATE TABLE IF NOT EXISTS experiment_runs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  experiment_id uuid NOT NULL REFERENCES experiments(id) ON DELETE CASCADE,
  dataset_id text NOT NULL,
  kernel_task_id text NULL,
  status text NOT NULL CHECK (status IN ('queued', 'running', 'done', 'error')),
  started_at timestamptz NULL,
  finished_at timestamptz NULL
);

CREATE INDEX IF NOT EXISTS idx_experiment_runs_experiment_id ON experiment_runs(experiment_id);
CREATE INDEX IF NOT EXISTS idx_experiment_runs_status ON experiment_runs(status);

-- 11) jobs
CREATE TABLE IF NOT EXISTS jobs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  kind text NOT NULL CHECK (kind IN ('tally', 'import_dataset', 'generate_dataset', 'experiment_run', 'report')),
  status text NOT NULL CHECK (status IN ('queued', 'running', 'done', 'error')),
  progress int NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),

  created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,

  election_id uuid NULL REFERENCES elections(id) ON DELETE SET NULL,
  experiment_id uuid NULL REFERENCES experiments(id) ON DELETE SET NULL,
  experiment_run_id uuid NULL REFERENCES experiment_runs(id) ON DELETE SET NULL,

  payload jsonb NULL,
  result_ref jsonb NULL,
  error_text text NULL,

  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz NULL,
  finished_at timestamptz NULL
);

-- notifications
CREATE TABLE IF NOT EXISTS notifications (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title text NOT NULL,
  message text NOT NULL,
  details text NULL,
  action_label text NULL,
  action_to text NULL,
  kind text NOT NULL CHECK (kind IN ('info', 'success', 'warning', 'error')),
  created_at timestamptz NOT NULL DEFAULT now(),
  read_at timestamptz NULL
);


CREATE TABLE IF NOT EXISTS admin_settings (
  id int PRIMARY KEY CHECK (id = 1),
  public_base_url text NULL,
  tls_mode text NOT NULL CHECK (tls_mode IN ('disabled', 'lets_encrypt', 'custom')),
  tls_domain text NULL,
  tls_contact_email text NULL,
  backup_enabled boolean NOT NULL DEFAULT false,
  backup_schedule text NULL,
  backup_retention_days int NULL CHECK (backup_retention_days IS NULL OR backup_retention_days > 0),
  database_host text NULL,
  database_name text NULL,
  updated_at timestamptz NOT NULL DEFAULT now(),
  updated_by uuid NULL REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_user_created_at ON notifications(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_user_read_at ON notifications(user_id, read_at);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_kind ON jobs(kind);
CREATE INDEX IF NOT EXISTS idx_jobs_election_id ON jobs(election_id);
CREATE INDEX IF NOT EXISTS idx_jobs_experiment_id ON jobs(experiment_id);
CREATE INDEX IF NOT EXISTS idx_jobs_experiment_run_id ON jobs(experiment_run_id);

COMMIT;
