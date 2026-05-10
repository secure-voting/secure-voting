import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import type { ApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { suffix } from "../src/ids.js";
import { selectExperimentRule } from "../src/rules.js";
import { waitSystemReady } from "../src/system.js";

test.setTimeout(240_000);

const rankingCandidates3 = [
  { id: "c1", name: "Alice" },
  { id: "c2", name: "Bob" },
  { id: "c3", name: "Carol" },
];

async function fetchDebugSnapshot(api: ApiClient, token: string, runId: string, jobId?: string) {
  const run = await api.rawGet(`/experiment-runs/${runId}`, token);
  const result = await api.rawGet(`/experiment-runs/${runId}/result`, token);
  const job = jobId ? await api.rawGet(`/jobs/${jobId}`, token) : null;

  return {
    run: run.body,
    result: result.body,
    job: job?.body ?? null,
  };
}

async function waitRunDone(
  api: ApiClient,
  token: string,
  runId: string,
  jobId?: string
) {
  const deadline = Date.now() + 90_000;
  let last: any = null;

  while (Date.now() < deadline) {
    last = await api.get<any>(`/experiment-runs/${runId}`, token);

    if (last.status === "done") {
      return last;
    }

    if (last.status === "error") {
      const snapshot = await fetchDebugSnapshot(api, token, runId, jobId);
      throw new Error(`experiment run failed: ${JSON.stringify(snapshot)}`);
    }

    await new Promise((resolve) => setTimeout(resolve, 2_000));
  }

  const snapshot = await fetchDebugSnapshot(api, token, runId, jobId);
  throw new Error(`experiment run timeout: ${JSON.stringify(snapshot)}`);
}

test.describe("experiment run through async compute pipeline", () => {
  test("researcher can generate dataset, create experiment, run compute and fetch result", async () => {
    const api = await createApiClient();
    const sfx = suffix();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);
      await waitSystemReady(api, admin.accessToken);

      const researcher = await api.login(env.researcherEmail, env.researcherPassword);
      const rule = await selectExperimentRule(api, researcher.accessToken, "ranking");

      const dataset = await api.post<any>(
        "/datasets/generate",
        {
          name: `E2E ranking dataset ${sfx}`,
          description: "structured experiment e2e dataset",
          format: "ranking",
          candidates: rankingCandidates3,
          voters: 20,
          seed: 42,
        },
        researcher.accessToken
      );

      expect(dataset.id).toBeTruthy();

      const experiment = await api.post<any>(
        "/experiments",
        {
          type: "algo",
          params: {
            ballot_format: "ranking",
            tally_rule: rule.id,
            committee_size: 1,
          },
          seed: 42,
        },
        researcher.accessToken
      );

      expect(experiment.id).toBeTruthy();

      const batch = await api.post<any>(
        "/experiment-runs/batch",
        {
          experiment_id: experiment.id,
          dataset_ids: [dataset.id],
        },
        researcher.accessToken
      );

      const first = Array.isArray(batch) ? batch[0] : batch.items?.[0] || batch.runs?.[0];
      const runId = first?.run_id || first?.id;
      const jobId = first?.job_id;

      expect(runId).toBeTruthy();

      await waitRunDone(api, researcher.accessToken, runId, jobId);

      const result = await api.get<any>(`/experiment-runs/${runId}/result`, researcher.accessToken);

      expect(result.run_id).toBe(runId);
      expect(Array.isArray(result.winners)).toBe(true);
    } finally {
      await api.dispose();
    }
  });
});