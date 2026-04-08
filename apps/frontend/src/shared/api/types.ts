export type APIErrorResponse = {
  error: {
    code: string;
    message: string;
  };
};

export type Me = {
  id?: string;
  email?: string;
  role?: "admin" | "voter" | "researcher" | string;
};

export type Candidate = {
  id: string;
  name: string;
  meta?: Record<string, unknown> | null;
};

export type CandidateDraft = {
  name: string;
  description: string;
};

export type CandidatePayload = {
  name: string;
  meta?: {
    description?: string;
  };
};

export type ImportedCandidate = {
  name: string;
  meta?: Record<string, unknown> | null;
};

export type InviteImportItem = {
  email: string;
  invite_id?: string;
  status?: string;
  code?: string;
};

export type InviteImportResponse = {
  total: number;
  parsed: number;
  created: InviteImportItem[];
  registration_required: InviteImportItem[];
  skipped: InviteImportItem[];
  failed: InviteImportItem[];
};

export type ElectionSummary = {
  id: string;
  title: string;
  description?: string | null;
  status: string;
  access_mode: string;
  start_at: string;
  end_at: string;
  published_at?: string | null;
  organizer_email?: string | null;
  ballot_format?: "approval" | "ranking" | "score" | string;
  tally_rule?: string;
  candidate_count?: number | null;
};

export type ElectionDetail = {
  id: string;
  title: string;
  description?: string | null;

  start_at: string;
  end_at: string;

  tally_rule: string;
  ballot_format: "approval" | "ranking" | "score" | string;

  committee_size?: number | null;
  quota_type?: string | null;

  status: string;
  access_mode: "open" | "invite" | string;

  publish_at?: string | null;
  published_at?: string | null;
  show_aggregates: boolean;

  approval_max_choices?: number | null;
  ranking_top_k?: number | null;

  score_min?: number | null;
  score_max?: number | null;
  score_step?: number | null;
  score_allow_skip: boolean;

  submitted_ballots_count?: number | null;

  invites_total_count?: number | null;
  invites_accepted_count?: number | null;
  invites_pending_count?: number | null;
  invites_revoked_count?: number | null;
  invites_failed_count?: number | null;
  invites_registration_required_count?: number | null;

  created_by?: string | null;
  created_at?: string | null;
  organizer_email?: string | null;

  candidates: Candidate[];
};

export type BallotMeta = {
  election_id: string;
  tally_rule: string;
  ballot_format: "approval" | "ranking" | "score" | string;

  approval_max_choices?: number | null;
  ranking_top_k?: number | null;

  score_min?: number | null;
  score_max?: number | null;
  score_step?: number | null;
  score_allow_skip: boolean;

  candidates: Candidate[];
};

export type MyBallotResp = {
  status: "none" | "draft" | "accepted" | "rejected" | string;
  submitted_at?: string | null;
  updated_at?: string | null;
};

export type ResultResp = {
  election_id: string;
  version: number;
  method: string;
  params?: unknown;
  winners: unknown;
  metrics?: unknown;
  protocol?: unknown;
  published_at?: string | null;
};

export type Invite = {
  id: string;
  email: string;
  status: string;
  sent_at?: string | null;
  accepted_at?: string | null;
  created_at: string;
};

export type InviteCreated = {
  invite_id: string;
  email: string;
  invite_code?: string;
  status: string;
  created_at: string;
  registration_required: boolean;
  registration_email_sent: boolean;
  invite_email_sent: boolean;
};

export type UpdateElectionRulesInput = {
  tally_rule?: string;
  ballot_format?: "approval" | "ranking" | "score";

  committee_size?: number;
  quota_type?: "hare" | "droop" | null;

  access_mode?: "open" | "invite";
  publish_at?: string | null;
  show_aggregates?: boolean;

  approval_max_choices?: number;
  ranking_top_k?: number | null;

  score_min?: number;
  score_max?: number;
  score_step?: number;
  score_allow_skip?: boolean;
};

export type DatasetListItem = {
  id: string;
  name: string;
  source: string;
  format: string;
  created_at: string;
};

export type DatasetDetail = {
  id: string;
  name: string;
  description?: string;
  source: string;
  format: string;
  candidates: Array<{
    id: string;
    name: string;
  }>;
  created_at: string;
  seed?: number | null;
  parameters?: Record<string, unknown>;
};

export type DatasetGenerateReq = {
  name: string;
  description?: string;
  format: "approval" | "ranking" | "score";
  candidates: Array<{
    id: string;
    name: string;
  }>;
  voters: number;
  seed?: number;
  generation_model?: "uniform" | "consensus" | "polarized" | string;

  approval_max_choices?: number;
  ranking_top_k?: number;
  score_min?: number;
  score_max?: number;
  score_step?: number;
};

export type Experiment = {
  id: string;
  type: string;
  params: unknown;
  status: string;
  seed?: number | null;
  created_by: string;
  created_at: string;
};

export type ExperimentCreateReq = {
  type: string;
  params?: Record<string, unknown>;
  seed?: number;
};

export type ExperimentRunItem = Record<string, unknown> & {
  id?: string;
  experiment_id?: string;
  dataset_id?: string;
  status?: string;
  started_at?: string | null;
  finished_at?: string | null;
};

export type JobItem = Record<string, unknown> & {
  id?: string;
  kind?: string;
  status?: string;
  progress?: number;
  created_at?: string;
  started_at?: string | null;
  finished_at?: string | null;
};

export type AuditLogItem = Record<string, unknown> & {
  id?: string | number;
  occurred_at?: string;
  event_type?: string;
};

export type TallyRuleInfo = {
  id: string;
  label: string;
  ballot_formats: Array<"approval" | "ranking" | "score" | string>;
  supports_election_tally: boolean;
  supports_experiment_runs: boolean;
  requires_committee_size: boolean;
  supports_quota_type: boolean;
  requires_approval_max_choices: boolean;
  supports_ranking_top_k: boolean;
  requires_score_range: boolean;
};

export type TallyRuleCapabilityView = TallyRuleInfo & {
  supports_ballot_format: (format: "approval" | "ranking" | "score") => boolean;
};