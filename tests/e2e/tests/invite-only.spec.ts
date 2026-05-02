import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";
import { candidates3 } from "../src/test-data.js";
import { daysFromNowIso, futureIso, suffix } from "../src/ids.js";

test.describe("invite-only elections", () => {
  test("invited voter can access ballot and ordinary voter is denied", async () => {
    const api = await createApiClient();
    const sfx = suffix();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);

      const invitedEmail = `invited_${sfx}@local.dev`;
      const invitedPassword = "invitedpass1";
      const ordinaryEmail = `ordinary_${sfx}@local.dev`;
      const ordinaryPassword = "ordinarypass1";

      await api.register(invitedEmail, invitedPassword);
      const ordinary = await api.register(ordinaryEmail, ordinaryPassword);

      const election = await api.post<any>(
        "/elections",
        {
          title: `E2E invite-only ${sfx}`,
          description: "invite-only playwright flow",
          start_at: futureIso(10),
          end_at: daysFromNowIso(2),
          tally_rule: "plurality",
          ballot_format: "ranking",
          access_mode: "invite",
          show_aggregates: true,
          committee_size: 1,
          ranking_top_k: 3,
          candidates: candidates3,
        },
        admin.accessToken
      );

      expect(election.id).toBeTruthy();

      await api.post(`/elections/${election.id}/actions/open`, undefined, admin.accessToken);

      const invite = await api.post<any>(
        `/elections/${election.id}/invites`,
        { email: invitedEmail },
        admin.accessToken
      );

      expect(invite.invite_code).toBeTruthy();

      const invited = await api.loginWithInvite(
        invitedEmail,
        invitedPassword,
        invite.invite_code
      );

      const invitedBallot = await api.get<any>(
        `/elections/${election.id}/ballot`,
        invited.accessToken
      );

      expect(invitedBallot.candidates).toHaveLength(3);

      const ordinaryResult = await api.rawGet(
        `/elections/${election.id}/ballot`,
        ordinary.accessToken
      );

      expect([403, 404]).toContain(ordinaryResult.status);
    } finally {
      await api.dispose();
    }
  });
});