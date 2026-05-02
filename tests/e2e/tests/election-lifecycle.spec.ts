import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import type { ApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { candidates3 } from "../src/test-data.js";
import { daysFromNowIso, futureIso, idempotencyKey, suffix } from "../src/ids.js";

async function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitUntilPublishSucceeds(
  api: ApiClient,
  token: string,
  electionId: string
): Promise<void> {
  const deadline = Date.now() + 120_000;
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

    throw new Error(`publish failed: HTTP ${result.status}: ${result.text}`);
  }

  throw new Error(`publish timeout: HTTP ${lastStatus}: ${lastText}`);
}

test.describe("election lifecycle", () => {
  test("admin creates, opens, closes and publishes ranking election", async () => {
    const api = await createApiClient();
    const sfx = suffix();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);
      const voter = await api.register(`voter_${sfx}@local.dev`, "voterpass1");

      const election = await api.post<any>(
        "/elections",
        {
          title: `E2E lifecycle ${sfx}`,
          description: "structured playwright e2e",
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

      await api.post(`/elections/${election.id}/actions/schedule`, undefined, admin.accessToken);
      await api.post(`/elections/${election.id}/actions/open`, undefined, admin.accessToken);

      const ballot = await api.get<any>(`/elections/${election.id}/ballot`, voter.accessToken);
      expect(ballot.candidates).toHaveLength(3);

      const ranking = ballot.candidates.map((candidate: any) => candidate.id);

      await api.post(
        `/elections/${election.id}/ballots/submit`,
        { ranking },
        voter.accessToken,
        { "Idempotency-Key": idempotencyKey() }
      );

      await api.post(`/elections/${election.id}/actions/close`, undefined, admin.accessToken);

      const hidden = await api.get<any>(
        `/elections/${election.id}/results`,
        voter.accessToken,
        403
      );

      expect(hidden.error.code).toBe("not_published");

      await waitUntilPublishSucceeds(api, admin.accessToken, election.id);

      const results = await api.get<any>(`/elections/${election.id}/results`, voter.accessToken);

      expect(results.published_at).toBeTruthy();
      expect(Array.isArray(results.winners)).toBe(true);
    } finally {
      await api.dispose();
    }
  });
});