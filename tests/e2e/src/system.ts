import type { ApiClient } from "./api.js";
import { sleep } from "./election.js";

function statusOf(value: unknown): string {
  if (!value || typeof value !== "object") return "";

  const rec = value as Record<string, unknown>;
  const status = rec.status;

  return typeof status === "string" ? status : "";
}

export async function waitSystemReady(
  api: ApiClient,
  token: string,
  timeoutMs = 90_000
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let last = "";

  while (Date.now() < deadline) {
    const result = await api.rawGet("/system/status", token);

    if (result.status === 200 && result.body && typeof result.body === "object") {
      const body = result.body as Record<string, unknown>;

      const backendStatus = statusOf(body.backend);
      const workerStatus = statusOf(body.worker);
      const computeStatus = statusOf(body.compute);

      last = JSON.stringify(body);

      if (
        backendStatus === "ready" &&
        workerStatus === "ready" &&
        (computeStatus === "ready" || computeStatus === "idle")
      ) {
        return;
      }
    } else {
      last = `HTTP ${result.status}: ${result.text}`;
    }

    await sleep(1_000);
  }

  throw new Error(`system is not ready for async e2e tests: ${last}`);
}