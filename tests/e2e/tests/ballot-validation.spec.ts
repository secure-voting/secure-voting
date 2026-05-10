import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { candidates3 } from "../src/test-data.js";
import { daysFromNowIso, futureIso, idempotencyKey, suffix } from "../src/ids.js";
import { selectElectionRule, type BallotFormat } from "../src/rules.js";

type ElectionSetup = {
  electionId: string;
  voterToken: string;
  candidateIds: string[];
};

async function createOpenElection(api: Awaited<ReturnType<typeof createApiClient>>, format: BallotFormat): Promise<ElectionSetup> {
  const sfx = suffix();
  const admin = await api.login(env.adminEmail, env.adminPassword);
  const voter = await api.register(`invalid_${format}_${sfx}@local.dev`, "voterpass1");
  const rule = await selectElectionRule(api, admin.accessToken, format);

  const electionReq: Record<string, unknown> = {
    title: `E2E invalid ${format} ${sfx}`,
    description: "negative ballot validation e2e",
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
  await api.post(`/elections/${election.id}/actions/open`, undefined, admin.accessToken);

  const ballot = await api.get<any>(`/elections/${election.id}/ballot`, voter.accessToken);
  const candidateIds = ballot.candidates.map((candidate: any) => candidate.id);

  return {
    electionId: election.id,
    voterToken: voter.accessToken,
    candidateIds,
  };
}

async function expectInvalidSubmit(
  api: Awaited<ReturnType<typeof createApiClient>>,
  setup: ElectionSetup,
  body: Record<string, unknown>
) {
  const result = await api.rawPost(
    `/elections/${setup.electionId}/ballots/submit`,
    body,
    setup.voterToken,
    { "Idempotency-Key": idempotencyKey() }
  );

  expect(result.status, `expected invalid ballot, got ${result.status}: ${result.text}`).toBe(400);
}

test.describe("ballot validation", () => {
  test("approval ballot rejects too many selected candidates", async () => {
    const api = await createApiClient();

    try {
      const setup = await createOpenElection(api, "approval");

      await expectInvalidSubmit(api, setup, {
        approval_set: setup.candidateIds,
      });
    } finally {
      await api.dispose();
    }
  });

  test("ranking ballot rejects duplicate and unknown candidates", async () => {
    const api = await createApiClient();

    try {
      const setup = await createOpenElection(api, "ranking");
      const [a, b] = setup.candidateIds;

      await expectInvalidSubmit(api, setup, {
        ranking: [a, a, b],
      });

      await expectInvalidSubmit(api, setup, {
        ranking: [a, b, "00000000-0000-0000-0000-000000000000"],
      });
    } finally {
      await api.dispose();
    }
  });

  test("score ballot rejects out-of-range, invalid step and unknown candidates", async () => {
    const api = await createApiClient();

    try {
      const setup = await createOpenElection(api, "score");
      const [a, b, c] = setup.candidateIds;

      await expectInvalidSubmit(api, setup, {
        scores: {
          [a]: 11,
          [b]: 6,
          [c]: 3,
        },
      });

      await expectInvalidSubmit(api, setup, {
        scores: {
          [a]: 9.5,
          [b]: 6,
          [c]: 3,
        },
      });

      await expectInvalidSubmit(api, setup, {
        scores: {
          [a]: 10,
          [b]: 6,
          "00000000-0000-0000-0000-000000000000": 3,
        },
      });
    } finally {
      await api.dispose();
    }
  });
});