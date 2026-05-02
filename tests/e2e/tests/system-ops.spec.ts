import { test, expect } from "@playwright/test";
import { createApiClient } from "../src/api.js";
import { env } from "../src/env.js";

function expectItemsArray(value: unknown, label: string) {
  if (Array.isArray(value)) {
    return;
  }

  if (value && typeof value === "object") {
    const rec = value as Record<string, unknown>;
    if (Array.isArray(rec.items)) {
      return;
    }
  }

  throw new Error(`${label} response does not contain array/items: ${JSON.stringify(value)}`);
}

test.describe("system operations", () => {
  test("admin can read system status, jobs and audit log", async () => {
    const api = await createApiClient();

    try {
      const admin = await api.login(env.adminEmail, env.adminPassword);

      const status = await api.get<any>("/system/status", admin.accessToken);

      expect(status.backend?.status).toBe("ready");
      expect(typeof status.compute?.status).toBe("string");
      expect(typeof status.worker?.status).toBe("string");
      expect(typeof status.checked_at).toBe("string");

      const jobs = await api.get<unknown>("/jobs", admin.accessToken);
      expectItemsArray(jobs, "jobs");

      const audit = await api.get<unknown>("/audit-log", admin.accessToken);
      expectItemsArray(audit, "audit-log");
    } finally {
      await api.dispose();
    }
  });
});