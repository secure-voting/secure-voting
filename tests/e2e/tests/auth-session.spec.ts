import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";

test.describe("auth and single-session policy", () => {
  test("login requires explicit session replacement when session already exists", async () => {
    const api = await createApiClient();

    try {
      const first = await api.post<any>("/auth/login", {
        email: env.adminEmail,
        password: env.adminPassword,
        replace_existing_session: true,
      });

      expect(first.access_token).toBeTruthy();

      const conflict = await api.post<any>(
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

      const replaced = await api.post<any>("/auth/login", {
        email: env.adminEmail,
        password: env.adminPassword,
        replace_existing_session: true,
      });

      expect(replaced.access_token).toBeTruthy();
      expect(replaced.user.role).toBe("admin");
    } finally {
      await api.dispose();
    }
  });
});