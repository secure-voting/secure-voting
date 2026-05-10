import crypto from "node:crypto";
import { login, register, request } from "./api.js";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

type BallotFormat = "approval" | "ranking" | "score";

type TallyRuleInfo = {
  id: string;
  label?: string;
  ballot_formats?: string[];
  supports_election_tally?: boolean;
  supports_experiment_runs?: boolean;
  requires_committee_size?: boolean;
  supports_quota_type?: boolean;
  requires_approval_max_choices?: boolean;
  supports_ranking_top_k?: boolean;
  requires_score_range?: boolean;
};

type ElectionResp = {
  id: string;
};

type BallotResp = {
  election_id?: string;
  ballot_format?: BallotFormat | string;
  candidates: Array<{
    id: string;
    name: string;
  }>;
};

type SubmitResult = {
  index: number;
  voterEmail: string;
  status: number;
  ok: boolean;
  durationMs: number;
  errorText?: string | undefined;
};

const ADMIN_EMAIL =
  process.env.ADMIN_EMAIL ||
  process.env.BOOTSTRAP_ADMIN_EMAIL ||
  "admin@example.com";

const ADMIN_PASSWORD =
  process.env.ADMIN_PASSWORD ||
  process.env.BOOTSTRAP_ADMIN_PASSWORD ||
  "AdminPass123!";

const BALLOT_FORMAT = parseBallotFormat(process.env.BALLOT_FORMAT || "ranking");
const VOTERS = parsePositiveInt(process.env.VOTERS || "10", "VOTERS");
const CONCURRENCY = parsePositiveInt(process.env.CONCURRENCY || "5", "CONCURRENCY");

const VOTER_PASSWORD = process.env.VOTER_PASSWORD || "LoadPass123!";
const TITLE_SUFFIX = crypto.randomUUID().slice(0, 8);

const candidates = [
  { name: "Alice", meta: { description: "Load test candidate A" } },
  { name: "Bob", meta: { description: "Load test candidate B" } },
  { name: "Carol", meta: { description: "Load test candidate C" } },
];

function parseBallotFormat(value: string): BallotFormat {
  const normalized = value.trim().toLowerCase();

  if (normalized === "approval" || normalized === "ranking" || normalized === "score") {
    return normalized;
  }

  throw new Error(`Unsupported BALLOT_FORMAT=${value}. Expected approval, ranking or score`);
}

function parsePositiveInt(value: string, name: string): number {
  const n = Number(value);

  if (!Number.isInteger(n) || n <= 0) {
    throw new Error(`${name} must be positive integer, got ${value}`);
  }

  return n;
}

function isoMinutesFromNow(minutes: number): string {
  return new Date(Date.now() + minutes * 60_000).toISOString();
}

function isoDaysFromNow(days: number): string {
  return new Date(Date.now() + days * 24 * 60 * 60_000).toISOString();
}

function percentile(values: number[], p: number): number {
  if (values.length === 0) return 0;

  const sorted = [...values].sort((a, b) => a - b);
  const index = Math.ceil((p / 100) * sorted.length) - 1;
  const safeIndex = Math.max(0, Math.min(sorted.length - 1, index));

  return sorted[safeIndex] ?? 0;
}

function average(values: number[]): number {
  if (values.length === 0) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}

async function getTallyRules(adminToken: string): Promise<TallyRuleInfo[]> {
  const resp = await request<{ items?: TallyRuleInfo[]; rules?: TallyRuleInfo[] } | TallyRuleInfo[]>(
    "GET",
    "/capabilities/tally-rules",
    {
      token: adminToken,
      expectedStatus: 200,
    }
  );

  if (Array.isArray(resp.body)) {
    return resp.body;
  }

  if (Array.isArray(resp.body.items)) {
    return resp.body.items;
  }

  if (Array.isArray(resp.body.rules)) {
    return resp.body.rules;
  }

  throw new Error(`Unexpected tally rules response: ${resp.text}`);
}

function selectRule(format: BallotFormat, rules: TallyRuleInfo[]): TallyRuleInfo {
  const compatible = rules.filter(
    (rule) =>
      rule.supports_election_tally &&
      Array.isArray(rule.ballot_formats) &&
      rule.ballot_formats.includes(format)
  );

  if (compatible.length === 0) {
    throw new Error(`No compatible election tally rule for ${format}: ${JSON.stringify(rules)}`);
  }

  const preferredByFormat: Record<BallotFormat, string[]> = {
    approval: ["approval-2", "approval-3"],
    ranking: ["plurality", "borda"],
    score: ["score"],
  };

  for (const preferred of preferredByFormat[format]) {
    const found = compatible.find((rule) => rule.id === preferred);
    if (found) return found;
  }

  const selected = compatible[0];
  if (!selected) {
    throw new Error(`No compatible election tally rule for ${format}: ${JSON.stringify(rules)}`);
  }

  return selected;
}

function createElectionPayload(format: BallotFormat, rule: TallyRuleInfo): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    title: `Load ${format} ${TITLE_SUFFIX}`,
    description: `Load test election for ${format} ballot submit`,
    start_at: isoMinutesFromNow(-5),
    end_at: isoDaysFromNow(2),
    tally_rule: rule.id,
    ballot_format: format,
    access_mode: "open",
    show_aggregates: true,
    committee_size: 1,
    candidates,
  };

  if (format === "approval") {
    payload.approval_max_choices = 2;
  }

  if (format === "ranking" && rule.supports_ranking_top_k) {
    payload.ranking_top_k = 3;
  }

  if (format === "score") {
    payload.score_min = 0;
    payload.score_max = 10;
    payload.score_step = 1;
    payload.score_allow_skip = true;
  }

  return payload;
}

async function createOpenElection(adminToken: string, format: BallotFormat): Promise<string> {
  const rules = await getTallyRules(adminToken);
  const rule = selectRule(format, rules);

  console.log(`selected rule: id=${rule.id} label=${rule.label || rule.id}`);

  const election = await request<ElectionResp>("POST", "/elections", {
    token: adminToken,
    body: createElectionPayload(format, rule),
    expectedStatus: 200,
  });

  await request("POST", `/elections/${election.body.id}/actions/open`, {
    token: adminToken,
    expectedStatus: 200,
  });

  return election.body.id;
}

function ballotSubmitBody(format: BallotFormat, candidateIds: string[]): Record<string, unknown> {
  const first = candidateIds[0];
  const second = candidateIds[1];
  const third = candidateIds[2];

  if (!first || !second || !third) {
    throw new Error(`Expected at least 3 candidates, got ${candidateIds.length}`);
  }

  if (format === "approval") {
    return {
      approval_set: [first, second],
    };
  }

  if (format === "ranking") {
    return {
      ranking: [first, second, third],
    };
  }

  return {
    scores: {
      [first]: 10,
      [second]: 7,
      [third]: 4,
    },
  };
}

async function setupVoter(index: number, electionId: string): Promise<{
  index: number;
  email: string;
  token: string;
  candidateIds: string[];
}> {
  const email = `load-${BALLOT_FORMAT}-${TITLE_SUFFIX}-${index}@local.dev`;

  const voter = await register(email, VOTER_PASSWORD);

  const ballot = await request<BallotResp>("GET", `/elections/${electionId}/ballot`, {
    token: voter.accessToken,
    expectedStatus: 200,
  });

  const candidateIds = ballot.body.candidates.map((candidate) => candidate.id);

  return {
    index,
    email,
    token: voter.accessToken,
    candidateIds,
  };
}

async function submitBallot(input: {
  index: number;
  email: string;
  token: string;
  electionId: string;
  candidateIds: string[];
}): Promise<SubmitResult> {
  const started = performance.now();

  try {
    const resp = await request("POST", `/elections/${input.electionId}/ballots/submit`, {
      token: input.token,
      body: ballotSubmitBody(BALLOT_FORMAT, input.candidateIds),
      headers: {
        "Idempotency-Key": crypto.randomUUID(),
      },
    });

    const durationMs = performance.now() - started;

    return {
      index: input.index,
      voterEmail: input.email,
      status: resp.status,
      ok: resp.status >= 200 && resp.status < 300,
      durationMs,
      errorText: resp.status >= 200 && resp.status < 300 ? undefined : resp.text,
    };
  } catch (error) {
    const durationMs = performance.now() - started;

    return {
      index: input.index,
      voterEmail: input.email,
      status: 0,
      ok: false,
      durationMs,
      errorText: error instanceof Error ? error.message : String(error),
    };
  }
}

async function runPool<T, R>(
  items: T[],
  concurrency: number,
  worker: (item: T) => Promise<R>
): Promise<R[]> {
  const results: R[] = [];
  let cursor = 0;

  async function runWorker() {
    while (cursor < items.length) {
      const index = cursor;
      cursor += 1;

      const item = items[index];
      if (item === undefined) {
        continue;
      }

      results[index] = await worker(item);
    }
  }

  const workers = Array.from(
    { length: Math.min(concurrency, items.length) },
    () => runWorker()
  );

  await Promise.all(workers);

  return results;
}

function printSummary(results: SubmitResult[], totalWallMs: number) {
  const success = results.filter((item) => item.ok);
  const failed = results.filter((item) => !item.ok);
  const durations = results.map((item) => item.durationMs);

  console.log("");
  console.log("== ballot load summary ==");
  console.log(`format: ${BALLOT_FORMAT}`);
  console.log(`voters: ${VOTERS}`);
  console.log(`concurrency: ${CONCURRENCY}`);
  console.log(`success: ${success.length}/${results.length}`);
  console.log(`failed: ${failed.length}/${results.length}`);
  console.log(`wall_ms: ${totalWallMs.toFixed(2)}`);
  console.log(`avg_ms: ${average(durations).toFixed(2)}`);
  console.log(`p50_ms: ${percentile(durations, 50).toFixed(2)}`);
  console.log(`p95_ms: ${percentile(durations, 95).toFixed(2)}`);
  console.log(`max_ms: ${Math.max(...durations).toFixed(2)}`);

  if (failed.length > 0) {
    console.log("");
    console.log("== failed submits ==");
    for (const item of failed.slice(0, 10)) {
      console.log(
        `#${item.index} ${item.voterEmail}: status=${item.status} duration_ms=${item.durationMs.toFixed(
          2
        )} error=${item.errorText || ""}`
      );
    }
  }
}

async function main() {
  console.log("== secure-voting ballot load ==");
  console.log(`format=${BALLOT_FORMAT}`);
  console.log(`voters=${VOTERS}`);
  console.log(`concurrency=${CONCURRENCY}`);
  console.log(`admin=${ADMIN_EMAIL}`);

  const admin = await login(ADMIN_EMAIL, ADMIN_PASSWORD);
  const electionId = await createOpenElection(admin.accessToken, BALLOT_FORMAT);

  console.log(`election_id=${electionId}`);
  console.log("setup voters...");

  const voters = [];
  for (let i = 0; i < VOTERS; i += 1) {
    voters.push(await setupVoter(i + 1, electionId));

    if ((i + 1) % 10 === 0 || i + 1 === VOTERS) {
      console.log(`prepared voters: ${i + 1}/${VOTERS}`);
    }
  }

  console.log("submit ballots...");

  const started = performance.now();

  const results = await runPool(
    voters.map((voter) => ({
      ...voter,
      electionId,
    })),
    CONCURRENCY,
    submitBallot
  );

  const totalWallMs = performance.now() - started;

  printSummary(results, totalWallMs);

  const failed = results.filter((item) => !item.ok);
  if (failed.length > 0) {
    process.exitCode = 1;
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});