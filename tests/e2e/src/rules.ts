import { expect } from "@playwright/test";
import type { ApiClient } from "./api.js";

export type BallotFormat = "approval" | "ranking" | "score";

export type TallyRuleInfo = {
  id: string;
  label: string;
  ballot_formats: string[];
  supports_election_tally: boolean;
  supports_experiment_runs: boolean;
  requires_committee_size: boolean;
  supports_quota_type: boolean;
  requires_approval_max_choices: boolean;
  supports_ranking_top_k: boolean;
  requires_score_range: boolean;
};

function supportsFormat(rule: TallyRuleInfo, format: BallotFormat): boolean {
  return Array.isArray(rule.ballot_formats) && rule.ballot_formats.includes(format);
}

function byStablePreference(format: BallotFormat) {
  const preferred: Record<BallotFormat, string[]> = {
    approval: ["approval_q2", "approval_q3", "approval"],
    ranking: ["plurality", "borda", "black", "copeland_i"],
    score: ["score_sum", "score_average", "threshold", "score"],
  };

  const order = preferred[format];

  return (a: TallyRuleInfo, b: TallyRuleInfo) => {
    const ai = order.indexOf(a.id);
    const bi = order.indexOf(b.id);

    if (ai >= 0 && bi >= 0) return ai - bi;
    if (ai >= 0) return -1;
    if (bi >= 0) return 1;

    return a.id.localeCompare(b.id);
  };
}

export async function loadTallyRules(api: ApiClient, token: string): Promise<TallyRuleInfo[]> {
  const value = await api.get<unknown>("/capabilities/tally-rules", token);

  if (Array.isArray(value)) {
    return value as TallyRuleInfo[];
  }

  if (value && typeof value === "object") {
    const rec = value as Record<string, unknown>;

    if (Array.isArray(rec.items)) {
      return rec.items as TallyRuleInfo[];
    }

    if (Array.isArray(rec.rules)) {
      return rec.rules as TallyRuleInfo[];
    }
  }

  throw new Error(`Unexpected tally rules response: ${JSON.stringify(value)}`);
}

export async function selectElectionRule(
  api: ApiClient,
  token: string,
  format: BallotFormat
): Promise<TallyRuleInfo> {
  const rules = await loadTallyRules(api, token);

  const candidates = rules
    .filter((rule) => rule.supports_election_tally)
    .filter((rule) => supportsFormat(rule, format))
    .sort(byStablePreference(format));

  expect(
    candidates.length,
    `No election tally rule found for ballot format ${format}. Available rules: ${JSON.stringify(rules)}`
  ).toBeGreaterThan(0);

  return candidates[0];
}

export async function selectExperimentRule(
  api: ApiClient,
  token: string,
  format: BallotFormat
): Promise<TallyRuleInfo> {
  const rules = await loadTallyRules(api, token);

  const candidates = rules
    .filter((rule) => rule.supports_experiment_runs)
    .filter((rule) => supportsFormat(rule, format))
    .sort(byStablePreference(format));

  expect(
    candidates.length,
    `No experiment rule found for ballot format ${format}. Available rules: ${JSON.stringify(rules)}`
  ).toBeGreaterThan(0);

  return candidates[0];
}