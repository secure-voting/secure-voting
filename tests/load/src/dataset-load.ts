import crypto from "node:crypto";
import { login, request } from "./api.js";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

type BallotFormat = "approval" | "ranking" | "score";

type DatasetResp = {
  id: string;
  name?: string;
  format?: string;
};

type LoadResult = {
  index: number;
  format: BallotFormat;
  status: number;
  ok: boolean;
  durationMs: number;
  datasetId?: string | undefined;
  errorText?: string | undefined;
};

const RESEARCHER_EMAIL =
  process.env.RESEARCHER_EMAIL ||
  process.env.BOOTSTRAP_RESEARCHER_EMAIL ||
  "researcher@example.com";

const RESEARCHER_PASSWORD =
  process.env.RESEARCHER_PASSWORD ||
  process.env.BOOTSTRAP_RESEARCHER_PASSWORD ||
  "ResearchPass123!";

const DATASET_FORMAT = parseDatasetFormat(process.env.DATASET_FORMAT || "all");
const DATASETS = parsePositiveInt(process.env.DATASETS || "6", "DATASETS");
const CONCURRENCY = parsePositiveInt(process.env.CONCURRENCY || "3", "CONCURRENCY");
const VOTERS = parsePositiveInt(process.env.DATASET_VOTERS || "20", "DATASET_VOTERS");
const TITLE_SUFFIX = crypto.randomUUID().slice(0, 8);

const candidates3 = [
  { id: "c1", name: "Alice" },
  { id: "c2", name: "Bob" },
  { id: "c3", name: "Carol" }
];

function parseDatasetFormat(value: string): BallotFormat | "all" {
  const normalized = value.trim().toLowerCase();

  if (
    normalized === "all" ||
    normalized === "approval" ||
    normalized === "ranking" ||
    normalized === "score"
  ) {
    return normalized;
  }

  throw new Error(`Unsupported DATASET_FORMAT=${value}. Expected all, approval, ranking or score`);
}

function parsePositiveInt(value: string, name: string): number {
  const n = Number(value);

  if (!Number.isInteger(n) || n <= 0) {
    throw new Error(`${name} must be positive integer, got ${value}`);
  }

  return n;
}

function formatsForRun(): BallotFormat[] {
  if (DATASET_FORMAT === "all") {
    return ["ranking", "approval", "score"];
  }

  return [DATASET_FORMAT];
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

function datasetPayload(format: BallotFormat, index: number): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    name: `Load dataset ${format} ${TITLE_SUFFIX}-${index}`,
    description: `Load generated dataset for ${format}`,
    format,
    candidates: candidates3,
    voters: VOTERS,
    seed: 10_000 + index,
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

async function generateDataset(input: {
  index: number;
  format: BallotFormat;
  token: string;
}): Promise<LoadResult> {
  const started = performance.now();

  try {
    const resp = await request<DatasetResp>("POST", "/datasets/generate", {
      token: input.token,
      body: datasetPayload(input.format, input.index)
    });

    const durationMs = performance.now() - started;

    return {
      index: input.index,
      format: input.format,
      status: resp.status,
      ok: resp.status >= 200 && resp.status < 300 && Boolean(resp.body.id),
      durationMs,
      datasetId: resp.body.id,
      errorText: resp.status >= 200 && resp.status < 300 ? undefined : resp.text
    };
  } catch (error) {
    const durationMs = performance.now() - started;

    return {
      index: input.index,
      format: input.format,
      status: 0,
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
  console.log("== dataset generate load summary ==");
  console.log(`format: ${DATASET_FORMAT}`);
  console.log(`datasets: ${results.length}`);
  console.log(`dataset_voters: ${VOTERS}`);
  console.log(`concurrency: ${CONCURRENCY}`);
  console.log(`success: ${success.length}/${results.length}`);
  console.log(`failed: ${failed.length}/${results.length}`);
  console.log(`wall_ms: ${wallMs.toFixed(2)}`);
  console.log(`avg_ms: ${average(durations).toFixed(2)}`);
  console.log(`p50_ms: ${percentile(durations, 50).toFixed(2)}`);
  console.log(`p95_ms: ${percentile(durations, 95).toFixed(2)}`);
  console.log(`max_ms: ${Math.max(...durations).toFixed(2)}`);

  const byFormat = new Map<BallotFormat, number>();
  for (const item of success) {
    byFormat.set(item.format, (byFormat.get(item.format) || 0) + 1);
  }

  console.log("");
  console.log("== success by format ==");
  for (const format of ["ranking", "approval", "score"] as BallotFormat[]) {
    console.log(`${format}: ${byFormat.get(format) || 0}`);
  }

  if (failed.length > 0) {
    console.log("");
    console.log("== failed datasets ==");
    for (const item of failed.slice(0, 10)) {
      console.log(
        `#${item.index} format=${item.format} status=${item.status} duration_ms=${item.durationMs.toFixed(
          2
        )} error=${item.errorText || ""}`
      );
    }
  }
}

async function main() {
  console.log("== secure-voting dataset generate load ==");
  console.log(`format=${DATASET_FORMAT}`);
  console.log(`datasets=${DATASETS}`);
  console.log(`dataset_voters=${VOTERS}`);
  console.log(`concurrency=${CONCURRENCY}`);
  console.log(`researcher=${RESEARCHER_EMAIL}`);

  const researcher = await login(RESEARCHER_EMAIL, RESEARCHER_PASSWORD);
  const formats = formatsForRun();

  const jobs = Array.from({ length: DATASETS }, (_, index) => ({
    index: index + 1,
    format: formats[index % formats.length] ?? "ranking",
    token: researcher.accessToken
  }));

  const started = performance.now();
  const results = await runPool(jobs, CONCURRENCY, generateDataset);
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