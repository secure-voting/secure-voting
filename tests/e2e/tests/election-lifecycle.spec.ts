import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { candidates3 } from "../src/test-data.js";
import { daysFromNowIso, futureIso, idempotencyKey, suffix } from "../src/ids.js";
import { waitUntilPublishSucceeds } from "../src/election.js";
import { waitSystemReady } from "../src/system.js";

test.setTimeout(240_000);

test.describe("election lifecycle", () => {
  test("admin creates, opens, closes and publishes ranking election", async () => {
    const api = await createApiClient();
    const sfx = suffix();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);

      await waitSystemReady(api, admin.accessToken);
      
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