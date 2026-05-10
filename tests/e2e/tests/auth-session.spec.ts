import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";

type AuthResponse = {
  access_token: string;
  expires_at: string;
  refresh_token: string;
  refresh_expires_at: string;
  user: {
    id: string;
    email: string;
    role: string;
  };
};

type ErrorEnvelope = {
  error: {
    code: string;
    message: string;
  };
};

type MeResponse = {
  id: string;
  email: string;
  role: string;
};

test.describe("auth and single-session policy", () => {
  test("login requires explicit session replacement when session already exists", async () => {
    const api = await createApiClient();

    try {
      const first = await api.post<AuthResponse>("/auth/login", {
        email: env.adminEmail,
        password: env.adminPassword,
        replace_existing_session: true,
      });

      expect(first.access_token).toBeTruthy();
      expect(first.refresh_token).toBeTruthy();

      const conflict = await api.post<ErrorEnvelope>(
        "/auth/login",
        {
          email: env.adminEmail,
          password: env.adminPassword,
        },
        undefined,
        undefined,
        409
      );

      expect(conflict.error.code).toBe("active_session_exists");

      const replaced = await api.post<AuthResponse>("/auth/login", {
        email: env.adminEmail,
        password: env.adminPassword,
        replace_existing_session: true,
      });

      expect(replaced.access_token).toBeTruthy();
      expect(replaced.refresh_token).toBeTruthy();
      expect(replaced.user.role).toBe("admin");
    } finally {
      await api.dispose();
    }
  });

  test("refresh rotates refresh token and invalidates previous access token", async () => {
    const api = await createApiClient();

    try {
      const first = await api.post<AuthResponse>("/auth/login", {
        email: env.adminEmail,
        password: env.adminPassword,
        replace_existing_session: true,
      });

      expect(first.access_token).toBeTruthy();
      expect(first.refresh_token).toBeTruthy();

      const refreshed = await api.post<AuthResponse>("/auth/refresh", {
        refresh_token: first.refresh_token,
      });

      expect(refreshed.access_token).toBeTruthy();
      expect(refreshed.refresh_token).toBeTruthy();
      expect(refreshed.access_token).not.toBe(first.access_token);
      expect(refreshed.refresh_token).not.toBe(first.refresh_token);
      expect(refreshed.user.email).toBe(env.adminEmail);
      expect(refreshed.user.role).toBe("admin");

      const oldAccessResult = await api.rawGet("/auth/me", first.access_token);
      expect(oldAccessResult.status, oldAccessResult.text).toBe(401);

      const oldRefreshResult = await api.post<ErrorEnvelope>(
        "/auth/refresh",
        {
          refresh_token: first.refresh_token,
        },
        undefined,
        undefined,
        401
      );

      expect(oldRefreshResult.error.code).toBe("unauthorized");
      expect(oldRefreshResult.error.message).toBe("invalid refresh token");

      const me = await api.get<MeResponse>("/auth/me", refreshed.access_token);

      expect(me.email).toBe(env.adminEmail);
      expect(me.role).toBe("admin");
    } finally {
      await api.dispose();
    }
  });
});