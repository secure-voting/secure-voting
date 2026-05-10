import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { loadTallyRules, type BallotFormat } from "../src/rules.js";

const formats: BallotFormat[] = ["approval", "ranking", "score"];

test.describe("capabilities and ballot rule compatibility", () => {
  test("compute capabilities expose compatible rules for all supported ballot formats", async () => {
    const api = await createApiClient();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);
      const rules = await loadTallyRules(api, admin.accessToken);

      expect(rules.length, `empty tally rules response: ${JSON.stringify(rules)}`).toBeGreaterThan(0);

      for (const format of formats) {
        const compatible = rules.filter(
          (rule) =>
            rule.supports_election_tally &&
            Array.isArray(rule.ballot_formats) &&
            rule.ballot_formats.includes(format)
        );

        expect(
          compatible.length,
          `No election tally rules for ${format}. Rules: ${JSON.stringify(rules)}`
        ).toBeGreaterThan(0);
      }

      const approvalRules = rules.filter((rule) => rule.ballot_formats?.includes("approval"));
      expect(
        approvalRules.some((rule) => rule.requires_approval_max_choices),
        `Approval rules must expose requires_approval_max_choices: ${JSON.stringify(approvalRules)}`
      ).toBe(true);

      const scoreRules = rules.filter((rule) => rule.ballot_formats?.includes("score"));
      expect(
        scoreRules.some((rule) => rule.requires_score_range),
        `Score rules must expose requires_score_range: ${JSON.stringify(scoreRules)}`
      ).toBe(true);

      for (const rule of scoreRules) {
        expect(rule.ballot_formats).toContain("score");
      }

      const rankingRules = rules.filter((rule) => rule.ballot_formats?.includes("ranking"));
      expect(
        rankingRules.length,
        `Expected ranking rules: ${JSON.stringify(rules)}`
      ).toBeGreaterThan(0);
    } finally {
      await api.dispose();
    }
  });
});