import type { ApiClient } from "./api.js";

export async function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function fetchElectionDebugSnapshot(
  api: ApiClient,
  token: string,
  electionId: string
): Promise<Record<string, unknown>> {
  const election = await api.rawGet(`/elections/${electionId}`, token);
  const jobs = await api.rawGet("/jobs", token);
  const results = await api.rawGet(`/elections/${electionId}/results`, token);

  return {
    election: election.body,
    jobs: jobs.body,
    results: results.body,
  };
}

export async function waitUntilPublishSucceeds(
  api: ApiClient,
  token: string,
  electionId: string,
  timeoutMs = 60_000
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  let lastStatus = 0;
  let lastText = "";

  while (Date.now() < deadline) {
    const result = await api.rawPost(
      `/elections/${electionId}/actions/publish`,
      undefined,
      token
    );

    lastStatus = result.status;
    lastText = result.text;

    if (result.status === 200) {
      return;
    }

    const body = result.body as any;
    const code = String(body?.error?.code || "");
    const message = String(body?.error?.message || "");

    const isResultNotReady =
      code === "no_results" ||
      code === "results_not_ready" ||
      code === "not_ready" ||
      message === "no_results" ||
      message === "results_not_ready" ||
      message === "not_ready" ||
      message.includes("no_results");

    if ((result.status === 400 || result.status === 409) && isResultNotReady) {
      await sleep(1_000);
      continue;
    }

    const snapshot = await fetchElectionDebugSnapshot(api, token, electionId);
    throw new Error(
      `publish failed: HTTP ${result.status}: ${result.text}; snapshot=${JSON.stringify(snapshot)}`
    );
  }

  const snapshot = await fetchElectionDebugSnapshot(api, token, electionId);
  throw new Error(
    `publish timeout: HTTP ${lastStatus}: ${lastText}; snapshot=${JSON.stringify(snapshot)}`
  );
}