import type {
  APIErrorResponse,
  AuthTokens,
  AuditLogItem,
  BallotMeta,
  DatasetDetail,
  DatasetGenerateReq,
  DatasetListItem,
  ElectionDetail,
  ElectionSummary,
  Experiment,
  ExperimentCreateReq,
  ExperimentRunItem,
  Invite,
  InviteCreated,
  JobItem,
  Me,
  MyBallotResp,
  ResultResp,
  TallyRuleInfo,
  CandidateDraft,
  ImportedCandidate,
  InviteImportResponse,
  SystemStatusResponse,
  NotificationCreateReq,
  NotificationItem,
  AdminUser,
  AdminSettings,
  AdminSettingsUpdateRequest,
  ScoreEntry,
  ProtocolStep,
  ExperimentRunResultResp,
  EmailVerificationRequestResult,
} from "./types";

const DEFAULT_TIMEOUT_MS = 15000;

function safeJsonParse(text: string): unknown | null {
  const t = text.trim();
  if (!t) return null;
  try {
    return JSON.parse(t) as unknown;
  } catch {
    return null;
  }
}

function isAPIErrorResponse(x: unknown): x is APIErrorResponse {
  if (!x || typeof x !== "object") return false;
  const o: any = x;
  return typeof o?.error?.code === "string" && typeof o?.error?.message === "string";
}

function buildQuery(params: Record<string, string | number | undefined | null>) {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null) continue;
    const str = String(value).trim();
    if (!str) continue;
    qs.set(key, str);
  }
  const out = qs.toString();
  return out ? `?${out}` : "";
}

export class ApiError extends Error {
  status: number;
  code: string;
  payload: unknown;

  constructor(status: number, code: string, message: string, payload: unknown) {
    super(message);
    this.status = status;
    this.code = code;
    this.payload = payload;
  }
}

async function request<T>(
  path: string,
  init: RequestInit & { timeoutMs?: number },
  token: string | null
): Promise<T> {
  const headers = new Headers(init.headers || {});
  headers.set("Accept", "application/json");

  const hasBody = init.body !== undefined && init.body !== null;
  if (hasBody && !headers.has("Content-Type") && typeof init.body === "string") {
    headers.set("Content-Type", "application/json");
  }
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const controller = new AbortController();
  const timeoutMs = typeof init.timeoutMs === "number" ? init.timeoutMs : DEFAULT_TIMEOUT_MS;
  const timer = window.setTimeout(() => controller.abort(), Math.max(1, timeoutMs));

  if (init.signal) {
    if (init.signal.aborted) controller.abort();
    else init.signal.addEventListener("abort", () => controller.abort(), { once: true });
  }

  try {
    const res = await fetch(path, { ...init, headers, signal: controller.signal });

    if (res.status === 204) return {} as T;

    const text = await res.text();
    const data = safeJsonParse(text);

    if (!res.ok) {
      if (isAPIErrorResponse(data)) {
        throw new ApiError(res.status, data.error.code, data.error.message, data);
      }
      throw new ApiError(res.status, "http_error", `HTTP ${res.status} ${res.statusText || "error"}`, data);
    }

    return (data ?? ({} as any)) as T;
  } finally {
    window.clearTimeout(timer);
  }
}

function extractToken(payload: unknown): string | null {
  if (!payload || typeof payload !== "object") return null;
  const p: any = payload;
  const candidates = [p.access_token, p.token, p.accessToken, p.jwt, p.data?.access_token, p.data?.token];
  for (const c of candidates) {
    if (typeof c === "string" && c.trim()) return c.trim();
  }
  return null;
}

function extractAuthTokens(payload: unknown): AuthTokens {
  if (!payload || typeof payload !== "object") {
    throw new Error("Ответ авторизации пустой или имеет неверный формат");
  }

  const p: any = payload;
  const accessToken = extractToken(payload);
  const refreshToken = typeof p.refresh_token === "string" ? p.refresh_token.trim() : "";
  const expiresAt = typeof p.expires_at === "string" ? p.expires_at : "";
  const refreshExpiresAt =
    typeof p.refresh_expires_at === "string" ? p.refresh_expires_at : "";

  if (!accessToken) {
    throw new Error("Авторизация выполнена, но access token не найден в ответе");
  }

  if (!refreshToken) {
    throw new Error("Авторизация выполнена, но refresh token не найден в ответе");
  }

  return {
    access_token: accessToken,
    expires_at: expiresAt,
    refresh_token: refreshToken,
    refresh_expires_at: refreshExpiresAt,
  };
}

function newIdempotencyKey(): string {
  const g: any = globalThis as any;
  const uuid =
    typeof g?.crypto?.randomUUID === "function"
      ? g.crypto.randomUUID()
      : `r${Math.random().toString(16).slice(2)}${Date.now().toString(16)}`;
  return `idem-${uuid}`;
}

async function authorizedDownload(path: string, token: string, fallbackFilename: string) {
  const res = await fetch(path, {
    method: "GET",
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  if (!res.ok) {
    const text = await res.text();
    const data = safeJsonParse(text);
    if (isAPIErrorResponse(data)) {
      throw new ApiError(res.status, data.error.code, data.error.message, data);
    }
    throw new ApiError(res.status, "http_error", `HTTP ${res.status}`, data);
  }

  const blob = await res.blob();
  const contentDisposition = res.headers.get("Content-Disposition") || "";
  const match = contentDisposition.match(/filename="?([^"]+)"?/i);
  const filename = match?.[1] || fallbackFilename;

  return { blob, filename };
}

export const api = {
  auth: {
    async register(email: string, password: string, inviteCode: string | null) {
      const body: Record<string, unknown> = { email, password };
      if (inviteCode && inviteCode.trim()) body.invite_code = inviteCode.trim();

      const resp = await request<any>(
        "/api/v1/auth/register",
        { method: "POST", body: JSON.stringify(body) },
        null
      );

      return extractAuthTokens(resp);
    },

    async login(
      email: string,
      password: string,
      inviteCode: string | null,
      replaceExistingSession = false
    ) {
      const body: Record<string, unknown> = { email, password };
      if (inviteCode && inviteCode.trim()) body.invite_code = inviteCode.trim();
      if (replaceExistingSession) body.replace_existing_session = true;

      const resp = await request<any>(
        "/api/v1/auth/login",
        { method: "POST", body: JSON.stringify(body) },
        null
      );

      return extractAuthTokens(resp);
    },

    async refresh(refreshToken: string) {
      const resp = await request<any>(
        "/api/v1/auth/refresh",
        {
          method: "POST",
          body: JSON.stringify({ refresh_token: refreshToken }),
        },
        null
      );

      return extractAuthTokens(resp);
    },

    async me(token: string, signal?: AbortSignal) {
      return await request<Me>("/api/v1/auth/me", { method: "GET", signal }, token);
    },

    async logout(token: string) {
      await request("/api/v1/auth/logout", { method: "POST", body: "{}" }, token);
    },

    async changePassword(token: string, currentPassword: string, newPassword: string) {
      return await request<{ ok: boolean }>(
        "/api/v1/auth/change-password",
        {
          method: "POST",
          body: JSON.stringify({
            current_password: currentPassword,
            new_password: newPassword,
          }),
        },
        token
      );
    },

    async updateProfile(token: string, fullName: string, phone: string) {
      return await request<Me>(
        "/api/v1/auth/profile",
        {
          method: "PATCH",
          body: JSON.stringify({
            full_name: fullName,
            phone,
          }),
        },
        token
      );
    },

    async requestEmailVerification(token: string) {
      return await request<EmailVerificationRequestResult>(
        "/api/v1/auth/email/verification/request",
        {
          method: "POST",
          body: "{}",
        },
        token
      );
    },

    async confirmEmailVerification(verificationToken: string) {
      return await request<Me>(
        "/api/v1/auth/email/verification/confirm",
        {
          method: "POST",
          body: JSON.stringify({
            token: verificationToken,
          }),
        },
        null
      );
    },
  },

  notifications: {
    async list(
      token: string,
      params?: {
        limit?: number;
        offset?: number;
      },
      signal?: AbortSignal
    ) {
      const query = buildQuery({
        limit: params?.limit,
        offset: params?.offset,
      });

      const resp = await request<{ items: NotificationItem[] }>(
        `/api/v1/notifications${query}`,
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async create(token: string, body: NotificationCreateReq) {
      return await request<NotificationItem>(
        "/api/v1/notifications",
        {
          method: "POST",
          body: JSON.stringify(body),
        },
        token
      );
    },

    async markRead(token: string, id: string) {
      return await request<{ ok: boolean }>(
        `/api/v1/notifications/${id}/read`,
        { method: "POST", body: "{}" },
        token
      );
    },

    async markAllRead(token: string) {
      return await request<{ ok: boolean }>(
        "/api/v1/notifications/read-all",
        { method: "POST", body: "{}" },
        token
      );
    },

    async remove(token: string, id: string) {
      return await request<{ ok: boolean }>(
        `/api/v1/notifications/${id}`,
        { method: "DELETE" },
        token
      );
    },

    async clearAll(token: string) {
      return await request<{ ok: boolean }>(
        "/api/v1/notifications",
        { method: "DELETE" },
        token
      );
    },
  },

  adminUsers: {
    async list(
      token: string,
      params?: {
        limit?: number;
        offset?: number;
      },
      signal?: AbortSignal
    ) {
      const query = buildQuery({
        limit: params?.limit,
        offset: params?.offset,
      });

      const resp = await request<{ items: AdminUser[] }>(
        `/api/v1/admin/users${query}`,
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async updateRole(token: string, userID: string, role: string) {
      return await request<AdminUser>(
        `/api/v1/admin/users/${userID}/role`,
        {
          method: "PATCH",
          body: JSON.stringify({ role }),
        },
        token
      );
    },
  },

  adminSettings: {
    async get(token: string, signal?: AbortSignal) {
      return await request<AdminSettings>(
        "/api/v1/admin/settings",
        { method: "GET", signal },
        token
      );
    },

    async update(token: string, body: AdminSettingsUpdateRequest) {
      return await request<AdminSettings>(
        "/api/v1/admin/settings",
        {
          method: "PUT",
          body: JSON.stringify(body),
        },
        token
      );
    },
  },

  system: {
    async status(token: string, signal?: AbortSignal) {
      return await request<SystemStatusResponse>(
        "/api/v1/system/status",
        { method: "GET", signal },
        token
      );
    },
  },

  capabilities: {
    async tallyRules(token: string, signal?: AbortSignal) {
      const resp = await request<{ items: TallyRuleInfo[] }>(
        "/api/v1/capabilities/tally-rules",
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },
  },

  elections: {
    list(token: string, signal?: AbortSignal) {
      return request<{ items: ElectionSummary[] }>(
        "/api/v1/elections",
        { method: "GET", signal },
        token
      ).then((resp) => (Array.isArray(resp.items) ? resp.items : []));
    },

    get(token: string, id: string, signal?: AbortSignal) {
      return request<ElectionDetail>(
        `/api/v1/elections/${id}`,
        { method: "GET", signal },
        token
      );
    },

    ballotMeta(token: string, id: string, signal?: AbortSignal) {
      return request<BallotMeta>(
        `/api/v1/elections/${id}/ballot`,
        { method: "GET", signal },
        token
      );
    },

    create(token: string, body: Record<string, unknown>) {
      return request<{ id: string }>(
        "/api/v1/elections",
        { method: "POST", body: JSON.stringify(body) },
        token
      ).then((resp) => resp.id);
    },

    updateRules(token: string, id: string, body: Record<string, unknown>) {
      return request<{ ok: boolean }>(
        `/api/v1/elections/${id}/rules`,
        { method: "PUT", body: JSON.stringify(body) },
        token
      );
    },

    action(token: string, id: string, action: string) {
      return request<{ ok: boolean }>(
        `/api/v1/elections/${id}/actions/${action}`,
        { method: "POST", body: "{}" },
        token
      );
    },

    listInvites(token: string, id: string, signal?: AbortSignal) {
      return request<{ items: Invite[] }>(
        `/api/v1/elections/${id}/invites`,
        {
          method: "GET",
          signal,
        },
        token
      ).then((resp) => (Array.isArray(resp.items) ? resp.items : []));
    },

    createInvite(token: string, id: string, email: string) {
      return request<InviteCreated>(
        `/api/v1/elections/${id}/invites`,
        {
          method: "POST",
          body: JSON.stringify({ email }),
        },
        token
      );
    },

    async importCandidates(token: string, file: File) {
      const form = new FormData();
      form.append("file", file);

      const res = await fetch("/api/v1/elections/candidates/import", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: form,
      });

      const text = await res.text();
      const data = safeJsonParse(text);

      if (!res.ok) {
        if (isAPIErrorResponse(data)) {
          throw new ApiError(res.status, data.error.code, data.error.message, data);
        }
        throw new ApiError(res.status, "http_error", `HTTP ${res.status}`, data);
      }

      const items = Array.isArray((data as { items?: ImportedCandidate[] } | null)?.items)
        ? ((data as { items?: ImportedCandidate[] }).items ?? [])
        : [];

      return items
        .map((item): CandidateDraft => {
          const meta = item?.meta && typeof item.meta === "object" ? item.meta : null;
          const description =
            meta && typeof meta.description === "string" ? meta.description : "";

          return {
            name: typeof item?.name === "string" ? item.name : "",
            description,
          };
        })
        .filter((item) => item.name.trim() !== "");
    },

    async importInvites(token: string, id: string, file: File) {
      const form = new FormData();
      form.append("file", file);

      const res = await fetch(`/api/v1/elections/${id}/invites/import`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: form,
      });

      const text = await res.text();
      const data = safeJsonParse(text);

      if (!res.ok) {
        if (isAPIErrorResponse(data)) {
          throw new ApiError(res.status, data.error.code, data.error.message, data);
        }
        throw new ApiError(res.status, "http_error", `HTTP ${res.status}`, data);
      }

      const parsed = (data ?? {}) as Partial<InviteImportResponse>;
      return {
        total: typeof parsed.total === "number" ? parsed.total : 0,
        parsed: typeof parsed.parsed === "number" ? parsed.parsed : 0,
        created: Array.isArray(parsed.created) ? parsed.created : [],
        registration_required: Array.isArray(parsed.registration_required) ? parsed.registration_required : [],
        skipped: Array.isArray(parsed.skipped) ? parsed.skipped : [],
        failed: Array.isArray(parsed.failed) ? parsed.failed : [],
      } satisfies InviteImportResponse;
    },
  },

  ballots: {
    async me(token: string, electionId: string, signal?: AbortSignal) {
      return await request<MyBallotResp>(
        `/api/v1/elections/${electionId}/ballots/me`,
        { method: "GET", signal },
        token
      );
    },

    async submit(token: string, electionId: string, body: any, idemKey: string) {
      return await request<any>(
        `/api/v1/elections/${electionId}/ballots/submit`,
        { method: "POST", headers: { "Idempotency-Key": idemKey }, body: JSON.stringify(body) },
        token
      );
    },

    newIdempotencyKey,
  },

  results: {
    async get(token: string, electionId: string, signal?: AbortSignal) {
      return await request<ResultResp>(`/api/v1/elections/${electionId}/results`, { method: "GET", signal }, token);
    },
  },

  datasets: {
    async list(token: string, signal?: AbortSignal) {
      const resp = await request<{ items: DatasetListItem[] }>(
        "/api/v1/datasets",
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async get(token: string, id: string, signal?: AbortSignal) {
      return await request<DatasetDetail>(`/api/v1/datasets/${id}`, { method: "GET", signal }, token);
    },

    async download(token: string, id: string) {
      return await authorizedDownload(`/api/v1/datasets/${id}/download`, token, `dataset-${id}`);
    },

    async importFile(
      token: string,
      payload: {
        name: string;
        description: string;
        format: string;
        file: File;
      }
    ) {
      const form = new FormData();
      form.append("name", payload.name);
      form.append("description", payload.description);
      form.append("format", payload.format);
      form.append("file", payload.file);

      const res = await fetch("/api/v1/datasets/import", {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: form,
      });

      const text = await res.text();
      const data = safeJsonParse(text);

      if (!res.ok) {
        if (isAPIErrorResponse(data)) {
          throw new ApiError(res.status, data.error.code, data.error.message, data);
        }
        throw new ApiError(res.status, "http_error", `HTTP ${res.status}`, data);
      }

      const parsed = (data ?? {}) as { id?: string };
      if (!parsed.id) throw new Error("Набор данных импортирован, но id не найден в ответе");
      return parsed.id;
    },

    async generate(token: string, body: DatasetGenerateReq) {
      const resp = await request<{ id: string }>(
        "/api/v1/datasets/generate",
        { method: "POST", body: JSON.stringify(body) },
        token
      );
      if (!resp?.id) throw new Error("Набор данных создан, но id не найден в ответе");
      return resp.id;
    },
  },

  experiments: {
    async list(token: string, signal?: AbortSignal) {
      const resp = await request<{ items: Experiment[] }>(
        "/api/v1/experiments",
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async get(token: string, id: string, signal?: AbortSignal) {
      return await request<Experiment>(`/api/v1/experiments/${id}`, { method: "GET", signal }, token);
    },

    async create(token: string, body: ExperimentCreateReq) {
      const resp = await request<{ id: string }>(
        "/api/v1/experiments",
        { method: "POST", body: JSON.stringify(body) },
        token
      );
      if (!resp?.id) throw new Error("Эксперимент создан, но id не найден в ответе");
      return resp.id;
    },
  },

  experimentRuns: {
    async batch(token: string, body: Record<string, unknown>) {
      const resp = await request<{ items: ExperimentRunItem[] }>(
        "/api/v1/experiment-runs/batch",
        { method: "POST", body: JSON.stringify(body) },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async list(
      token: string,
      params?: {
        experiment_id?: string;
      },
      signal?: AbortSignal
    ) {
      const query = buildQuery({
        experiment_id: params?.experiment_id,
      });

      const resp = await request<{ items: ExperimentRunItem[] }>(
        `/api/v1/experiment-runs${query}`,
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async get(token: string, id: string, signal?: AbortSignal) {
      return await request<ExperimentRunItem>(`/api/v1/experiment-runs/${id}`, { method: "GET", signal }, token);
    },

    async result(token: string, id: string, signal?: AbortSignal) {
      return await request<ExperimentRunResultResp>(
        `/api/v1/experiment-runs/${id}/result`,
        { method: "GET", signal },
        token
      );
    },

    async download(token: string, id: string) {
      return await authorizedDownload(`/api/v1/experiment-runs/${id}/download`, token, `experiment-run-${id}`);
    },
  },

  jobs: {
    async list(
      token: string,
      params?: {
        status?: string;
        kind?: string;
        limit?: number;
        offset?: number;
      },
      signal?: AbortSignal
    ) {
      const query = buildQuery({
        status: params?.status,
        kind: params?.kind,
        limit: params?.limit,
        offset: params?.offset,
      });

      const resp = await request<{ items: JobItem[] }>(
        `/api/v1/jobs${query}`,
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },

    async get(token: string, id: string, signal?: AbortSignal) {
      return await request<JobItem>(`/api/v1/jobs/${id}`, { method: "GET", signal }, token);
    },
  },

  audit: {
    async list(
      token: string,
      params?: {
        event_type?: string;
        actor_user_id?: string;
        since?: string;
        until?: string;
        limit?: number;
        offset?: number;
      },
      signal?: AbortSignal
    ) {
      const query = buildQuery({
        event_type: params?.event_type,
        actor_user_id: params?.actor_user_id,
        since: params?.since,
        until: params?.until,
        limit: params?.limit,
        offset: params?.offset,
      });

      const resp = await request<{ items: AuditLogItem[] }>(
        `/api/v1/audit-log${query}`,
        { method: "GET", signal },
        token
      );
      return Array.isArray(resp.items) ? resp.items : [];
    },
  },
};