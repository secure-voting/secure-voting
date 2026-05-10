import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { candidates3 } from "../src/test-data.js";
import { daysFromNowIso, futureIso, idempotencyKey, suffix } from "../src/ids.js";
import { BallotFormat, selectElectionRule } from "../src/rules.js";

const cases: BallotFormat[] = ["approval", "ranking", "score"];

test.describe("ballot formats", () => {
  for (const format of cases) {
    test(`${format} ballot can be submitted`, async () => {
      const api = await createApiClient();
      const sfx = suffix();

      try {
        const admin = await api.login(env.adminEmail, env.adminPassword);
        const voter = await api.register(`voter_${format}_${sfx}@local.dev`, "voterpass1");
        const rule = await selectElectionRule(api, admin.accessToken, format);

        const electionReq: Record<string, unknown> = {
          title: `E2E ${format} ${sfx}`,
          description: "format e2e",
          start_at: futureIso(10),
          end_at: daysFromNowIso(2),
          tally_rule: rule.id,
          ballot_format: format,
          access_mode: "open",
          show_aggregates: true,
          committee_size: 1,
          candidates: candidates3,
        };

        if (format === "approval") {
          electionReq.approval_max_choices = 2;
        }

        if (format === "ranking" && rule.supports_ranking_top_k) {
          electionReq.ranking_top_k = 3;
        }

        if (format === "score") {
          electionReq.score_min = 0;
          electionReq.score_max = 10;
          electionReq.score_step = 1;
          electionReq.score_allow_skip = true;
        }

        const election = await api.post<any>("/elections", electionReq, admin.accessToken);

        expect(election.id).toBeTruthy();

        await api.post(`/elections/${election.id}/actions/open`, undefined, admin.accessToken);

        const ballot = await api.get<any>(`/elections/${election.id}/ballot`, voter.accessToken);
        const ids = ballot.candidates.map((candidate: any) => candidate.id);

        let submitBody: Record<string, unknown>;

        if (format === "approval") {
          submitBody = { approval_set: ids.slice(0, 2) };
        } else if (format === "ranking") {
          submitBody = { ranking: ids };
        } else {
          submitBody = {
            scores: {
              [ids[0]]: 10,
              [ids[1]]: 6,
              [ids[2]]: 3,
            },
          };
        }

        const submitted = await api.post<any>(
          `/elections/${election.id}/ballots/submit`,
          submitBody,
          voter.accessToken,
          { "Idempotency-Key": idempotencyKey() }
        );

        expect(submitted.ballot_id || submitted.id || submitted.ok).toBeTruthy();
      } finally {
        await api.dispose();
      }
    });
  }
});