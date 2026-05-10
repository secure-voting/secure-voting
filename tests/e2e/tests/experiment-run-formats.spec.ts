import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import type { ApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { suffix } from "../src/ids.js";
import { selectExperimentRule, type BallotFormat } from "../src/rules.js";
import { waitSystemReady } from "../src/system.js";

test.setTimeout(240_000);

const candidates3 = [
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

async function waitRunDone(api: ApiClient, token: string, runId: string, jobId?: string) {
  const deadline = Date.now() + 90_000;

  while (Date.now() < deadline) {
    const run = await api.get<any>(`/experiment-runs/${runId}`, token);

    if (run.status === "done") {
      return run;
    }

    if (run.status === "error") {
      const snapshot = await fetchDebugSnapshot(api, token, runId, jobId);
      throw new Error(`experiment run failed: ${JSON.stringify(snapshot)}`);
    }

    await new Promise((resolve) => setTimeout(resolve, 2_000));
  }

  const snapshot = await fetchDebugSnapshot(api, token, runId, jobId);
  throw new Error(`experiment run timeout: ${JSON.stringify(snapshot)}`);
}

function datasetPayload(format: BallotFormat, sfx: string): Record<string, unknown> {
  const payload: Record<string, unknown> = {
    name: `E2E ${format} compute dataset ${sfx}`,
    description: `compute pipeline dataset for ${format}`,
    format,
    candidates: candidates3,
    voters: 20,
    seed: 42,
    generation_model: "uniform",
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
    committee_size: 1,
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

test.describe("experiment run formats", () => {
  for (const format of ["ranking", "approval", "score"] as BallotFormat[]) {
    test(`${format} experiment can run through compute pipeline`, async () => {
      const api = await createApiClient();
      const sfx = suffix();

      try {
        const admin = await api.login(env.adminEmail, env.adminPassword);
        await waitSystemReady(api, admin.accessToken);

        const researcher = await api.login(env.researcherEmail, env.researcherPassword);
        const rule = await selectExperimentRule(api, researcher.accessToken, format);

        const dataset = await api.post<any>(
          "/datasets/generate",
          datasetPayload(format, sfx),
          researcher.accessToken
        );

        expect(dataset.id).toBeTruthy();

        const experiment = await api.post<any>(
          "/experiments",
          {
            type: "algo",
            params: experimentParams(format, rule.id),
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
        expect(result.status || "done").toBe("done");
        expect(Array.isArray(result.winners)).toBe(true);
        expect(result.metrics).toBeTruthy();
        expect(result.timings).toBeTruthy();
      } finally {
        await api.dispose();
      }
    });
  }
});