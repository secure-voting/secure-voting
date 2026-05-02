export type AuthSession = {
  accessToken: string;
  user: {
    id: string;
    email: string;
    role: string;
  };
};

export type ApiResponse<T> = {
  status: number;
  body: T;
  text: string;
};

const API_BASE = process.env.API_BASE || "https://127.0.0.1:8080/api/v1";

function normalizePath(path: string): string {
  return path.startsWith("/") ? path : `/${path}`;
}

function parseJson(text: string): unknown {
  if (!text) return null;

  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function errorCode(body: unknown): string {
  if (!body || typeof body !== "object") return "";

  const rec = body as Record<string, unknown>;
  const err = rec.error;

  if (!err || typeof err !== "object") return "";

  const errRec = err as Record<string, unknown>;
  return typeof errRec.code === "string" ? errRec.code : "";
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function request<T>(
  method: "GET" | "POST",
  path: string,
  options: {
    token?: string;
    body?: unknown;
    headers?: Record<string, string>;
    expectedStatus?: number;
  } = {}
): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };

  if (options.token) {
    headers.Authorization = `Bearer ${options.token}`;
  }

  const init: RequestInit = {
    method,
    headers,
  };

  if (options.body !== undefined) {
    init.body = JSON.stringify(options.body);
  }

  const resp = await fetch(`${API_BASE}${normalizePath(path)}`, init);

  const text = await resp.text();
  const body = parseJson(text) as T;

  if (options.expectedStatus !== undefined && resp.status !== options.expectedStatus) {
    throw new Error(`${method} ${path}: expected ${options.expectedStatus}, got ${resp.status}: ${text}`);
  }

  return {
    status: resp.status,
    body,
    text,
  };
}

async function postWithRateLimitRetry<T>(
  path: string,
  body: unknown,
  expectedStatus = 200
): Promise<ApiResponse<T>> {
  const delays = [0, 5_000, 10_000, 20_000, 40_000, 60_000, 90_000, 120_000];
  let last: ApiResponse<T> | null = null;

  for (const delay of delays) {
    if (delay > 0) {
      await sleep(delay);
    }

    const result = await request<T>("POST", path, {
      body,
    });

    last = result;

    if (result.status !== 429 || errorCode(result.body) !== "rate_limited") {
      if (result.status !== expectedStatus) {
        throw new Error(`POST ${path}: expected ${expectedStatus}, got ${result.status}: ${result.text}`);
      }

      return result;
    }
  }

  throw new Error(`POST ${path}: rate limit retry exhausted: ${last?.text || "no response"}`);
}

export async function login(email: string, password: string): Promise<AuthSession> {
  const result = await postWithRateLimitRetry<any>("/auth/login", {
    email,
    password,
    replace_existing_session: true,
  });

  return {
    accessToken: result.body.access_token,
    user: result.body.user,
  };
}

export async function register(email: string, password: string): Promise<AuthSession> {
  const result = await postWithRateLimitRetry<any>("/auth/register", {
    email,
    password,
  });

  return {
    accessToken: result.body.access_token,
    user: result.body.user,
  };
}