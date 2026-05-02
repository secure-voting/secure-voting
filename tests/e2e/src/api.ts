import { expect, request } from "@playwright/test";
import { env } from "./env.js";

export type AuthSession = {
  accessToken: string;
  refreshToken?: string;
  user: {
    id: string;
    email: string;
    role: string;
    full_name?: string | null;
    email_verified?: boolean;
  };
};

export type ApiClient = Awaited<ReturnType<typeof createApiClient>>;

type RawResponse = {
  status: number;
  body: unknown;
  text: string;
};

function withTrailingSlash(value: string): string {
  return value.endsWith("/") ? value : `${value}/`;
}

function apiPath(path: string): string {
  return path.replace(/^\/+/, "");
}

function parseBody(text: string): unknown {
  if (!text) return null;

  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function errorCode(value: unknown): string {
  if (!value || typeof value !== "object") return "";

  const rec = value as Record<string, unknown>;
  const err = rec.error;

  if (!err || typeof err !== "object") return "";

  const errRec = err as Record<string, unknown>;
  return typeof errRec.code === "string" ? errRec.code : "";
}

function shouldRetryRateLimit(path: string): boolean {
  const normalized = `/${apiPath(path)}`;

  return (
    normalized === "/auth/register" ||
    normalized === "/auth/login" ||
    normalized === "/auth/refresh" ||
    normalized === "/auth/email/verification/request" ||
    normalized === "/auth/email/verification/confirm" ||
    normalized === "/datasets/generate" ||
    normalized === "/datasets/import" ||
    normalized === "/experiments" ||
    normalized === "/experiment-runs/batch" ||
    normalized === "/elections"
  );
}

export async function createApiClient() {
  const ctx = await request.newContext({
    baseURL: withTrailingSlash(env.apiBase),
    ignoreHTTPSErrors: true,
    extraHTTPHeaders: {
      "Content-Type": "application/json",
    },
  });

  async function rawGet(path: string, token?: string): Promise<RawResponse> {
    const resp = await ctx.get(apiPath(path), {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    });

    const text = await resp.text();

    return {
      status: resp.status(),
      body: parseBody(text),
      text,
    };
  }

  async function rawPost(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>
  ): Promise<RawResponse> {
    const resp = await ctx.post(apiPath(path), {
      data: body,
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(extraHeaders || {}),
      },
    });

    const text = await resp.text();

    return {
      status: resp.status(),
      body: parseBody(text),
      text,
    };
  }

  async function rawPatch(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>
  ): Promise<RawResponse> {
    const resp = await ctx.patch(apiPath(path), {
      data: body,
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(extraHeaders || {}),
      },
    });

    const text = await resp.text();

    return {
      status: resp.status(),
      body: parseBody(text),
      text,
    };
  }

  async function rawPostWithRateLimitRetry(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>
  ): Promise<RawResponse> {
    const delaysMs = [0, 1_000, 2_000, 5_000, 10_000, 20_000, 30_000];

    let last: RawResponse | null = null;

    for (const delay of delaysMs) {
      if (delay > 0) {
        await sleep(delay);
      }

      const result = await rawPost(path, body, token, extraHeaders);
      last = result;

      if (result.status !== 429 || errorCode(result.body) !== "rate_limited") {
        return result;
      }
    }

    return last as RawResponse;
  }

  async function rawPostWithWriteRetry(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>
  ): Promise<RawResponse> {
    const delaysMs = [0, 5_000, 10_000, 20_000, 40_000, 60_000, 90_000, 120_000];

    let last: RawResponse | null = null;

    for (const delay of delaysMs) {
      if (delay > 0) {
        await sleep(delay);
      }

      const result = await rawPost(path, body, token, extraHeaders);
      last = result;

      if (result.status !== 429 || errorCode(result.body) !== "rate_limited") {
        return result;
      }
    }

    return last as RawResponse;
  }

  async function post<T>(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>,
    expectedStatus = 200
  ): Promise<T> {
    const result = shouldRetryRateLimit(path)
      ? await rawPostWithWriteRetry(path, body, token, extraHeaders)
      : await rawPost(path, body, token, extraHeaders);

    expect(result.status, `${path}: ${result.text}`).toBe(expectedStatus);
    return result.body as T;
  }

  async function postWithWriteRetry<T>(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>,
    expectedStatus = 200
  ): Promise<T> {
    const result = await rawPostWithWriteRetry(path, body, token, extraHeaders);
    expect(result.status, `${path}: ${result.text}`).toBe(expectedStatus);
    return result.body as T;
  }

  async function patch<T>(
    path: string,
    body?: unknown,
    token?: string,
    extraHeaders?: Record<string, string>,
    expectedStatus = 200
  ): Promise<T> {
    const result = await rawPatch(path, body, token, extraHeaders);
    expect(result.status, `${path}: ${result.text}`).toBe(expectedStatus);
    return result.body as T;
  }

  async function get<T>(
    path: string,
    token?: string,
    expectedStatus = 200
  ): Promise<T> {
    const result = await rawGet(path, token);
    expect(result.status, `${path}: ${result.text}`).toBe(expectedStatus);
    return result.body as T;
  }

  async function login(email: string, password: string): Promise<AuthSession> {
    const result = await rawPostWithRateLimitRetry("/auth/login", {
      email,
      password,
      replace_existing_session: true,
    });

    expect(result.status, `/auth/login: ${result.text}`).toBe(200);

    const body = result.body as any;

    return {
      accessToken: body.access_token,
      refreshToken: body.refresh_token,
      user: body.user,
    };
  }

  async function loginWithInvite(email: string, password: string, inviteCode: string): Promise<AuthSession> {
    const result = await rawPostWithRateLimitRetry("/auth/login", {
      email,
      password,
      invite_code: inviteCode,
      replace_existing_session: true,
    });

    expect(result.status, `/auth/login: ${result.text}`).toBe(200);

    const body = result.body as any;

    return {
      accessToken: body.access_token,
      refreshToken: body.refresh_token,
      user: body.user,
    };
  }

  async function register(email: string, password: string): Promise<AuthSession> {
    const result = await rawPostWithRateLimitRetry("/auth/register", {
      email,
      password,
    });

    expect(result.status, `/auth/register: ${result.text}`).toBe(200);

    const body = result.body as any;

    return {
      accessToken: body.access_token,
      refreshToken: body.refresh_token,
      user: body.user,
    };
  }

  async function dispose() {
    await ctx.dispose();
  }

  return {
    ctx,
    rawGet,
    rawPost,
    rawPatch,
    rawPostWithRateLimitRetry,
    rawPostWithWriteRetry,
    get,
    post,
    postWithWriteRetry,
    patch,
    login,
    loginWithInvite,
    register,
    dispose,
  };
}