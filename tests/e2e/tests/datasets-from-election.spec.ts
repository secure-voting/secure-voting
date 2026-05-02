import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { candidates3 } from "../src/test-data.js";
import { daysFromNowIso, futureIso, idempotencyKey, suffix } from "../src/ids.js";
import { waitUntilPublishSucceeds } from "../src/election.js";
import { waitSystemReady } from "../src/system.js";

test.setTimeout(240_000);

function responseId(value: unknown): string {
  if (!value || typeof value !== "object") return "";

  const rec = value as Record<string, unknown>;

  if (typeof rec.id === "string") return rec.id;
  if (typeof rec.dataset_id === "string") return rec.dataset_id;

  return "";
}

test.describe("datasets from real elections", () => {
  test("published election ballots can be converted to research dataset", async () => {
    const api = await createApiClient();
    const sfx = suffix();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);
      await waitSystemReady(api, admin.accessToken);
      const voter = await api.register(`dataset_voter_${sfx}@local.dev`, "voterpass1");

      const election = await api.post<any>(
        "/elections",
        {
          title: `E2E dataset from election ${sfx}`,
          description: "dataset from real election playwright flow",
          start_at: futureIso(10),
          end_at: daysFromNowIso(2),
          tally_rule: "plurality",
          ballot_format: "ranking",
          access_mode: "open",
          show_aggregates: true,
          committee_size: 1,
          ranking_top_k: 3,
          candidates: candidates3,
        },
        admin.accessToken
      );

      expect(election.id).toBeTruthy();

      await api.post(`/elections/${election.id}/actions/open`, undefined, admin.accessToken);

      const ballot = await api.get<any>(`/elections/${election.id}/ballot`, voter.accessToken);
      const ranking = ballot.candidates.map((candidate: any) => candidate.id);

      await api.post(
        `/elections/${election.id}/ballots/submit`,
        { ranking },
        voter.accessToken,
        { "Idempotency-Key": idempotencyKey() }
      );

      await api.post(`/elections/${election.id}/actions/close`, undefined, admin.accessToken);
      await waitUntilPublishSucceeds(api, admin.accessToken, election.id);

      const createdDataset = await api.post<any>(
        "/datasets/from-election",
        {
          election_id: election.id,
          name: `Dataset from election ${sfx}`,
          description: "created by Playwright E2E",
        },
        admin.accessToken
      );

      const datasetId = responseId(createdDataset);
      expect(datasetId).toBeTruthy();

      const dataset = await api.get<any>(`/datasets/${datasetId}`, admin.accessToken);

      expect(dataset.id).toBe(datasetId);
      expect(dataset.source).toBe("election");
      expect(dataset.format).toBe("ranking");
      expect(Array.isArray(dataset.candidates)).toBe(true);
      expect(dataset.candidates.length).toBeGreaterThanOrEqual(3);
    } finally {
      await api.dispose();
    }
  });
});