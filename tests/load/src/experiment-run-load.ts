import crypto from "node:crypto";
import { login, request } from "./api.js";

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

type DatasetResp = {
  id: string;
};

type ExperimentResp = {
  id: string;
};

type ExperimentRunResp = {
  id?: string;
  run_id?: string;
  job_id?: string;
  status?: string;
  kernel_task_id?: string;
};

type LoadResult = {
  index: number;
  format: BallotFormat;
  ruleId: string;
  ok: boolean;
  durationMs: number;
  runId?: string | undefined;
  jobId?: string | undefined;
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

const RESEARCHER_EMAIL =
  process.env.RESEARCHER_EMAIL ||
  process.env.BOOTSTRAP_RESEARCHER_EMAIL ||
  "researcher@example.com";

const RESEARCHER_PASSWORD =
  process.env.RESEARCHER_PASSWORD ||
  process.env.BOOTSTRAP_RESEARCHER_PASSWORD ||
  "ResearchPass123!";

const FORMATS = parseFormats(process.env.EXPERIMENT_FORMATS || "ranking,approval,score");
const RUNS_PER_FORMAT = parsePositiveInt(process.env.RUNS_PER_FORMAT || "1", "RUNS_PER_FORMAT");
const CONCURRENCY = parsePositiveInt(process.env.CONCURRENCY || "1", "CONCURRENCY");
const DATASET_VOTERS = parsePositiveInt(process.env.DATASET_VOTERS || "20", "DATASET_VOTERS");
const WAIT_TIMEOUT_MS = parsePositiveInt(process.env.WAIT_TIMEOUT_MS || "120000", "WAIT_TIMEOUT_MS");

const TITLE_SUFFIX = crypto.randomUUID().slice(0, 8);

const candidates3 = [
  { id: "c1", name: "Alice" },
  { id: "c2", name: "Bob" },
  { id: "c3", name: "Carol" }
];

function parseFormats(value: string): BallotFormat[] {
  const raw = value
    .split(",")
    .map((item) => item.trim().toLowerCase())
    .filter(Boolean);

  const formats: BallotFormat[] = [];

  for (const item of raw) {
    if (item !== "ranking" && item !== "approval" && item !== "score") {
      throw new Error(`Unsupported EXPERIMENT_FORMATS item=${item}`);
    }

    formats.push(item);
  }

  if (formats.length === 0) {
    throw new Error("EXPERIMENT_FORMATS must contain at least one format");
  }

  return formats;
}

function parsePositiveInt(value: string, name: string): number {
  const n = Number(value);

  if (!Number.isInteger(n) || n <= 0) {
    throw new Error(`${name} must be positive integer, got ${value}`);
  }

  return n;
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

function supportsFormat(rule: TallyRuleInfo, format: BallotFormat): boolean {
  return Array.isArray(rule.ballot_formats) && rule.ballot_formats.includes(format);
}

function preferredRuleIds(format: BallotFormat): string[] {
  if (format === "approval") return ["approval-2", "approval-3", "approval"];
  if (format === "score") return ["score"];
  return ["plurality", "borda", "black"];
}

async function getTallyRules(token: string): Promise<TallyRuleInfo[]> {
  const resp = await request<{ items?: TallyRuleInfo[]; rules?: TallyRuleInfo[] } | TallyRuleInfo[]>(
    "GET",
    "/capabilities/tally-rules",
    {
      token,
      expectedStatus: 200
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

function selectExperimentRule(format: BallotFormat, rules: TallyRuleInfo[]): TallyRuleInfo {
  const compatible = rules.filter(
    (rule) => rule.supports_experiment_runs && supportsFormat(rule, format)
  );

  if (compatible.length === 0) {
    throw new Error(`No compatible experiment rule for ${format}: ${JSON.stringify(rules)}`);
  }

  for (const id of preferredRuleIds(format)) {
    const found = compatible.find((rule) => rule.id === id);
    if (found) return found;
  }

  const selected = compatible[0];
  if (!selected) {
    throw new Error(`No compatible experiment rule for ${format}: ${JSON.stringify(rules)}`);
  }

  return selected;
}

function datasetPayload(format: BallotFormat, index: number): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    name: `Load experiment dataset ${format} ${TITLE_SUFFIX}-${index}`,
    description: `Load experiment dataset for ${format}`,
    format,
    candidates: candidates3,
    voters: DATASET_VOTERS,
    seed: 50_000 + index,
    generation_model: "uniform"
  };

  if (format === "approval") {
    payload.approval_max_choices = 2;
  }

  if (format === "ranking") {
    payload.ranking_top_k = 3;
  }

  if (format === "score") {
    payload.score_min = 0;
    payload.score_max = 10;
    payload.score_step = 1;
  }

  return payload;
}

function experimentParams(format: BallotFormat, ruleId: string): Record<string, unknown> {
  const params: Record<string, unknown> = {
    ballot_format: format,
    tally_rule: ruleId,
    committee_size: 1
  };

  if (format === "approval") {
    params.approval_max_choices = 2;
  }

  if (format === "ranking") {
    params.ranking_top_k = 3;
  }

  if (format === "score") {
    params.score_min = 0;
    params.score_max = 10;
    params.score_step = 1;
    params.score_allow_skip = true;
  }

  return params;
}

function firstRunFromBatch(value: unknown): ExperimentRunResp {
  if (Array.isArray(value)) {
    return (value[0] ?? {}) as ExperimentRunResp;
  }

  if (value && typeof value === "object") {
    const rec = value as Record<string, unknown>;

    if (Array.isArray(rec.items)) {
      return (rec.items[0] ?? {}) as ExperimentRunResp;
    }

    if (Array.isArray(rec.runs)) {
      return (rec.runs[0] ?? {}) as ExperimentRunResp;
    }
  }

  return {};
}

async function waitRunDone(token: string, runId: string): Promise<void> {
  const deadline = Date.now() + WAIT_TIMEOUT_MS;

  while (Date.now() < deadline) {
    const run = await request<ExperimentRunResp>("GET", `/experiment-runs/${runId}`, {
      token,
      expectedStatus: 200
    });

    if (run.body.status === "done") {
      return;
    }

    if (run.body.status === "error") {
      throw new Error(`experiment run failed: ${JSON.stringify(run.body)}`);
    }

    await new Promise((resolve) => setTimeout(resolve, 2_000));
  }

  throw new Error(`experiment run timeout: run_id=${runId}`);
}

async function runExperiment(input: {
  index: number;
  format: BallotFormat;
  token: string;
  rule: TallyRuleInfo;
}): Promise<LoadResult> {
  const started = performance.now();

  try {
    const dataset = await request<DatasetResp>("POST", "/datasets/generate", {
      token: input.token,
      body: datasetPayload(input.format, input.index),
      expectedStatus: 200
    });

    const experiment = await request<ExperimentResp>("POST", "/experiments", {
      token: input.token,
      body: {
        type: "algo",
        params: experimentParams(input.format, input.rule.id),
        seed: 42
      },
      expectedStatus: 200
    });

    const batch = await request<unknown>("POST", "/experiment-runs/batch", {
      token: input.token,
      body: {
        experiment_id: experiment.body.id,
        dataset_ids: [dataset.body.id]
      },
      expectedStatus: 200
    });

    const first = firstRunFromBatch(batch.body);
    const runId = first.run_id || first.id || "";
    const jobId = first.job_id || first.kernel_task_id || "";

    if (!runId) {
      throw new Error(`batch response does not contain run id: ${batch.text}`);
    }

    await waitRunDone(input.token, runId);

    const result = await request<Record<string, unknown>>("GET", `/experiment-runs/${runId}/result`, {
      token: input.token,
      expectedStatus: 200
    });

    const durationMs = performance.now() - started;

    return {
      index: input.index,
      format: input.format,
      ruleId: input.rule.id,
      ok: Boolean(result.body.run_id),
      durationMs,
      runId,
      jobId,
      errorText: result.body.run_id ? undefined : `empty result: ${result.text}`
    };
  } catch (error) {
    const durationMs = performance.now() - started;

    return {
      index: input.index,
      format: input.format,
      ruleId: input.rule.id,
      ok: false,
      durationMs,
      errorText: error instanceof Error ? error.message : String(error)
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

function printSummary(results: LoadResult[], wallMs: number) {
  const success = results.filter((item) => item.ok);
  const failed = results.filter((item) => !item.ok);
  const durations = results.map((item) => item.durationMs);

  console.log("");
  console.log("== experiment run load summary ==");
  console.log(`formats: ${FORMATS.join(",")}`);
  console.log(`runs_per_format: ${RUNS_PER_FORMAT}`);
  console.log(`dataset_voters: ${DATASET_VOTERS}`);
  console.log(`concurrency: ${CONCURRENCY}`);
  console.log(`success: ${success.length}/${results.length}`);
  console.log(`failed: ${failed.length}/${results.length}`);
  console.log(`wall_ms: ${wallMs.toFixed(2)}`);
  console.log(`avg_ms: ${average(durations).toFixed(2)}`);
  console.log(`p50_ms: ${percentile(durations, 50).toFixed(2)}`);
  console.log(`p95_ms: ${percentile(durations, 95).toFixed(2)}`);
  console.log(`max_ms: ${Math.max(...durations).toFixed(2)}`);

  console.log("");
  console.log("== success by format ==");
  for (const format of ["ranking", "approval", "score"] as BallotFormat[]) {
    console.log(`${format}: ${success.filter((item) => item.format === format).length}`);
  }

  if (failed.length > 0) {
    console.log("");
    console.log("== failed experiment runs ==");
    for (const item of failed.slice(0, 10)) {
      console.log(
        `#${item.index} format=${item.format} rule=${item.ruleId} duration_ms=${item.durationMs.toFixed(
          2
        )} error=${item.errorText || ""}`
      );
    }
  }
}

async function main() {
  console.log("== secure-voting experiment run load ==");
  console.log(`formats=${FORMATS.join(",")}`);
  console.log(`runs_per_format=${RUNS_PER_FORMAT}`);
  console.log(`dataset_voters=${DATASET_VOTERS}`);
  console.log(`concurrency=${CONCURRENCY}`);
  console.log(`admin=${ADMIN_EMAIL}`);
  console.log(`researcher=${RESEARCHER_EMAIL}`);

  await login(ADMIN_EMAIL, ADMIN_PASSWORD);
  const researcher = await login(RESEARCHER_EMAIL, RESEARCHER_PASSWORD);
  const rules = await getTallyRules(researcher.accessToken);

  const jobs: Array<{
    index: number;
    format: BallotFormat;
    token: string;
    rule: TallyRuleInfo;
  }> = [];

  let index = 1;

  for (const format of FORMATS) {
    const rule = selectExperimentRule(format, rules);

    for (let i = 0; i < RUNS_PER_FORMAT; i += 1) {
      jobs.push({
        index,
        format,
        token: researcher.accessToken,
        rule
      });
      index += 1;
    }
  }

  const started = performance.now();
  const results = await runPool(jobs, CONCURRENCY, runExperiment);
  const wallMs = performance.now() - started;

  printSummary(results, wallMs);

  if (results.some((item) => !item.ok)) {
    process.exitCode = 1;
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});