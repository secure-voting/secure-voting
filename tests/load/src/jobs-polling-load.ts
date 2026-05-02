import { login, request } from "./api.js";

process.env.NODE_TLS_REJECT_UNAUTHORIZED = "0";

type PollResult = {
  index: number;
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

const REQUESTS = parsePositiveInt(process.env.REQUESTS || "50", "REQUESTS");
const CONCURRENCY = parsePositiveInt(process.env.CONCURRENCY || "10", "CONCURRENCY");
const PATH = process.env.JOBS_PATH || "/jobs";

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

async function pollJobs(input: {
  index: number;
  token: string;
}): Promise<PollResult> {
  const started = performance.now();

  try {
    const resp = await request<unknown>("GET", PATH, {
      token: input.token
    });

    const durationMs = performance.now() - started;

    return {
      index: input.index,
      status: resp.status,
      ok: resp.status >= 200 && resp.status < 300,
      durationMs,
      errorText: resp.status >= 200 && resp.status < 300 ? undefined : resp.text
    };
  } catch (error) {
    const durationMs = performance.now() - started;

    return {
      index: input.index,
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

function printSummary(results: PollResult[], wallMs: number) {
  const success = results.filter((item) => item.ok);
  const failed = results.filter((item) => !item.ok);
  const durations = results.map((item) => item.durationMs);

  console.log("");
  console.log("== jobs polling load summary ==");
  console.log(`path: ${PATH}`);
  console.log(`requests: ${REQUESTS}`);
  console.log(`concurrency: ${CONCURRENCY}`);
  console.log(`success: ${success.length}/${results.length}`);
  console.log(`failed: ${failed.length}/${results.length}`);
  console.log(`wall_ms: ${wallMs.toFixed(2)}`);
  console.log(`avg_ms: ${average(durations).toFixed(2)}`);
  console.log(`p50_ms: ${percentile(durations, 50).toFixed(2)}`);
  console.log(`p95_ms: ${percentile(durations, 95).toFixed(2)}`);
  console.log(`max_ms: ${Math.max(...durations).toFixed(2)}`);

  if (failed.length > 0) {
    console.log("");
    console.log("== failed polls ==");
    for (const item of failed.slice(0, 10)) {
      console.log(
        `#${item.index} status=${item.status} duration_ms=${item.durationMs.toFixed(
          2
        )} error=${item.errorText || ""}`
      );
    }
  }
}

async function main() {
  console.log("== secure-voting jobs polling load ==");
  console.log(`path=${PATH}`);
  console.log(`requests=${REQUESTS}`);
  console.log(`concurrency=${CONCURRENCY}`);
  console.log(`admin=${ADMIN_EMAIL}`);

  const admin = await login(ADMIN_EMAIL, ADMIN_PASSWORD);

  const jobs = Array.from({ length: REQUESTS }, (_, index) => ({
    index: index + 1,
    token: admin.accessToken
  }));

  const started = performance.now();
  const results = await runPool(jobs, CONCURRENCY, pollJobs);
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