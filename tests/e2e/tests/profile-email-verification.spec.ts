import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { suffix } from "../src/ids.js";

type MeResponse = {
  id: string;
  email: string;
  role: string;
  full_name?: string | null;
  phone?: string | null;
  email_verified?: boolean;
  email_verified_at?: string | null;
};

type EmailVerificationRequestResult = {
  ok: boolean;
  already_verified: boolean;
  delivery?: "dev" | "smtp" | string;
  expires_at?: string;
  max_attempts?: number;
  verification_code?: string;
};

test.describe("profile and email verification", () => {
  test("user can update full name and verify email with one-time code", async () => {
    const api = await createApiClient();

    try {
      const email = `profile-email-${suffix()}@example.com`;
      const password = "StrongPass123!";
      const fullName = "Тестовый Пользователь";
      const phone = "+7 (999) 000-00-00";

      const registered = await api.register(email, password);

      expect(registered.accessToken).toBeTruthy();
      expect(registered.refreshToken).toBeTruthy();
      expect(registered.user.email).toBe(email);
      expect(registered.user.role).toBe("voter");

      const updated = await api.patch<MeResponse>(
        "/auth/profile",
        {
          full_name: fullName,
          phone,
        },
        registered.accessToken
      );

      expect(updated.email).toBe(email);
      expect(updated.full_name).toBe(fullName);
      expect(updated.phone).toBe(phone);

      const meAfterProfile = await api.get<MeResponse>("/auth/me", registered.accessToken);

      expect(meAfterProfile.email).toBe(email);
      expect(meAfterProfile.full_name).toBe(fullName);
      expect(meAfterProfile.phone).toBe(phone);

      const verification = await api.post<EmailVerificationRequestResult>(
        "/auth/email/verification/request",
        {},
        registered.accessToken
      );

      expect(verification.ok).toBe(true);

      if (verification.already_verified) {
        const me = await api.get<MeResponse>("/auth/me", registered.accessToken);
        expect(me.email_verified).toBe(true);
        return;
      }

      expect(verification.verification_code).toBeTruthy();

      const confirmed = await api.post<MeResponse>(
        "/auth/email/verification/confirm",
        {
          code: verification.verification_code,
        },
        registered.accessToken
      );

      expect(confirmed.email).toBe(email);
      expect(confirmed.email_verified).toBe(true);
      expect(confirmed.email_verified_at).toBeTruthy();
      expect(confirmed.full_name).toBe(fullName);

      const meAfterConfirm = await api.get<MeResponse>("/auth/me", registered.accessToken);

      expect(meAfterConfirm.email_verified).toBe(true);
      expect(meAfterConfirm.email_verified_at).toBeTruthy();
      expect(meAfterConfirm.full_name).toBe(fullName);
      expect(meAfterConfirm.phone).toBe(phone);
    } finally {
      await api.dispose();
    }
  });
});