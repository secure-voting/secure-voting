import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";

type AnyObj = Record<string, unknown>;

type Me = {
  id?: string;
  email?: string;
  role?: "admin" | "voter" | "researcher" | string;
};

type ElectionSummary = {
  id: string;
  title: string;
  description?: string | null;
  status: string;
  access_mode: string;
  start_at: string;
  end_at: string;
  published_at?: string | null;
};

type ElectionDetail = {
  id: string;
  title: string;
  description?: string | null;

  start_at: string;
  end_at: string;

  tally_rule: string;
  ballot_format: "approval" | "ranking" | "score" | string;

  committee_size?: number | null;
  quota_type?: string | null;

  status: string;
  access_mode: "open" | "invite" | string;

  publish_at?: string | null;
  published_at?: string | null;
  show_aggregates: boolean;

  approval_max_choices?: number | null;
  ranking_top_k?: number | null;

  score_min?: number | null;
  score_max?: number | null;
  score_step?: number | null;
  score_allow_skip: boolean;

  candidates: Array<{ id: string; name: string; meta?: AnyObj | null }>;
};

type BallotMeta = {
  election_id: string;
  tally_rule: string;
  ballot_format: "approval" | "ranking" | "score" | string;
  approval_max_choices?: number | null;
  ranking_top_k?: number | null;
  score_min?: number | null;
  score_max?: number | null;
  score_step?: number | null;
  score_allow_skip: boolean;
  candidates: Array<{ id: string; name: string; meta?: AnyObj | null }>;
};

type MyBallotResp = {
  status: "none" | "draft" | "accepted" | "rejected" | string;
  submitted_at?: string | null;
  updated_at?: string | null;
};

type ResultResp = {
  election_id: string;
  version: number;
  method: string;
  params?: unknown;
  winners: unknown;
  metrics?: unknown;
  protocol?: unknown;
  published_at?: string | null;
};

type Invite = {
  id: string;
  email: string;
  status: string;
  sent_at?: string | null;
  accepted_at?: string | null;
  created_at: string;
};

type InviteCreated = {
  invite_id: string;
  email: string;
  invite_code: string;
  status: string;
  created_at: string;
};

const STORAGE_TOKEN_KEY = "sv_token";
const DEFAULT_TIMEOUT_MS = 15000;
const IS_DEV = Boolean((import.meta as any)?.env?.DEV);

function nowRfc3339Plus(minutes: number) {
  const d = new Date(Date.now() + minutes * 60_000);
  return d.toISOString();
}

function readStoredToken(): string | null {
  const t = localStorage.getItem(STORAGE_TOKEN_KEY);
  return t && t.trim() ? t.trim() : null;
}

function writeStoredToken(token: string) {
  localStorage.setItem(STORAGE_TOKEN_KEY, token);
}

function clearStoredToken() {
  localStorage.removeItem(STORAGE_TOKEN_KEY);
}

function safeJsonParse(text: string): unknown | null {
  const t = text.trim();
  if (!t) return null;
  try {
    return JSON.parse(t) as unknown;
  } catch {
    return null;
  }
}

function extractToken(payload: unknown): string | null {
  if (!payload || typeof payload !== "object") return null;
  const p = payload as any;
  const candidates = [
    p.access_token,
    p.token,
    p.accessToken,
    p.jwt,
    p.data?.access_token,
    p.data?.token,
  ];
  for (const c of candidates) {
    if (typeof c === "string" && c.trim()) return c.trim();
  }
  return null;
}

function formatApiError(payload: unknown, fallback: string): string {
  if (!payload) return fallback;
  if (typeof payload === "string" && payload.trim()) return payload.trim();

  if (typeof payload === "object") {
    const p = payload as any;
    const direct =
      p.message ||
      p.error ||
      p.code ||
      p.detail ||
      p.reason ||
      p?.error?.message ||
      p?.error?.code ||
      p?.error?.error ||
      p?.error?.detail;

    if (typeof direct === "string" && direct.trim()) return direct.trim();

    try {
      return JSON.stringify(payload);
    } catch {
      return fallback;
    }
  }

  return fallback;
}

function newIdempotencyKey(): string {
  const g = globalThis as any;
  const uuid =
    typeof g?.crypto?.randomUUID === "function"
      ? g.crypto.randomUUID()
      : `r${Math.random().toString(16).slice(2)}${Date.now().toString(16)}`;
  return `idem-${uuid}`;
}

async function apiRequest<T>(
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
      const msg = formatApiError(data, `HTTP ${res.status} ${res.statusText || "error"}`);
      const err = new Error(msg) as Error & { status?: number; payload?: unknown };
      err.status = res.status;
      err.payload = data;
      throw err;
    }

    return (data ?? ({} as any)) as T;
  } finally {
    window.clearTimeout(timer);
  }
}

function useHashRouter() {
  const [hash, setHash] = useState(() => window.location.hash || "#/");

  useEffect(() => {
    const onHash = () => setHash(window.location.hash || "#/");
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, []);

  const route = useMemo(() => {
    const raw = (hash || "#/").replace(/^#/, "");
    const parts = raw.split("/").filter(Boolean);

    if (parts.length === 0) return { name: "home" as const };

    if (parts[0] === "login") return { name: "login" as const };
    if (parts[0] === "elections" && parts.length === 1) return { name: "elections" as const };
    if (parts[0] === "elections" && parts.length === 2) return { name: "election" as const, id: parts[1] };
    if (parts[0] === "elections" && parts.length === 3 && parts[2] === "vote") return { name: "vote" as const, id: parts[1] };
    if (parts[0] === "elections" && parts.length === 3 && parts[2] === "results") return { name: "results" as const, id: parts[1] };
    if (parts[0] === "admin" && parts[1] === "create") return { name: "admin_create" as const };

    return { name: "not_found" as const, raw };
  }, [hash]);

  const go = useCallback((to: string) => {
    if (!to.startsWith("#")) window.location.hash = `#${to}`;
    else window.location.hash = to;
  }, []);

  return { route, go };
}

const styles: Record<string, React.CSSProperties> = {
  page: {
    fontFamily: "system-ui, -apple-system, Segoe UI, Roboto, sans-serif",
    padding: 20,
    maxWidth: 1100,
    margin: "0 auto",
  },
  topbar: {
    display: "flex",
    alignItems: "center",
    gap: 12,
    justifyContent: "space-between",
    padding: "10px 12px",
    border: "1px solid #e5e7eb",
    borderRadius: 12,
    marginBottom: 16,
    background: "white",
  },
  title: { margin: 0, fontSize: 20 },
  btn: {
    padding: "8px 12px",
    borderRadius: 10,
    border: "1px solid #d1d5db",
    background: "white",
    cursor: "pointer",
  },
  btnPrimary: {
    padding: "8px 12px",
    borderRadius: 10,
    border: "1px solid #111827",
    background: "#111827",
    color: "white",
    cursor: "pointer",
  },
  btnDanger: {
    padding: "8px 12px",
    borderRadius: 10,
    border: "1px solid #991b1b",
    background: "#991b1b",
    color: "white",
    cursor: "pointer",
  },
  input: {
    padding: "8px 10px",
    borderRadius: 10,
    border: "1px solid #d1d5db",
    width: "100%",
    boxSizing: "border-box",
    background: "white",
  },
  card: {
    border: "1px solid #e5e7eb",
    borderRadius: 12,
    padding: 12,
    background: "white",
  },
  grid2: {
    display: "grid",
    gridTemplateColumns: "1fr 1fr",
    gap: 12,
  },
  muted: { color: "#6b7280" },
  hr: { border: "none", borderTop: "1px solid #e5e7eb", margin: "12px 0" },
  pre: {
    whiteSpace: "pre-wrap",
    wordBreak: "break-word",
    background: "#0b1020",
    color: "#e5e7eb",
    padding: 12,
    borderRadius: 12,
    fontSize: 12,
    overflowX: "auto",
  },
};

function Badge({ text }: { text: string }) {
  return (
    <span
      style={{
        display: "inline-block",
        padding: "2px 8px",
        borderRadius: 999,
        border: "1px solid #e5e7eb",
        fontSize: 12,
      }}
    >
      {text}
    </span>
  );
}

function ErrorBanner({ error }: { error: string | null }) {
  if (!error) return null;
  return (
    <div
      style={{
        ...styles.card,
        borderColor: "#fecaca",
        background: "#fff1f2",
        color: "#7f1d1d",
        marginBottom: 12,
      }}
    >
      <b>Ошибка:</b> {error}
    </div>
  );
}

function JsonBlock({ value }: { value: unknown }) {
  return <pre style={styles.pre}>{JSON.stringify(value, null, 2)}</pre>;
}

export default function App() {
  const { route, go } = useHashRouter();

  const [token, setToken] = useState<string | null>(() => readStoredToken());
  const [me, setMe] = useState<Me | null>(null);

  const [bootLoading, setBootLoading] = useState(false);
  const [bootError, setBootError] = useState<string | null>(null);

  const authed = Boolean(token && me?.email);

  const clearAuth = useCallback(() => {
    clearStoredToken();
    setToken(null);
    setMe(null);
  }, []);

  const bootAbortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    bootAbortRef.current?.abort();
    bootAbortRef.current = null;

    if (!token) {
      setMe(null);
      setBootError(null);
      setBootLoading(false);
      return;
    }

    const ac = new AbortController();
    bootAbortRef.current = ac;

    const run = async () => {
      setBootLoading(true);
      setBootError(null);

      try {
        const resp = await apiRequest<any>("/api/v1/auth/me", { method: "GET", signal: ac.signal }, token);
        const obj = resp?.user ?? resp?.data?.user ?? resp?.data ?? resp ?? {};
        const nextMe: Me = {
          id: obj.id ?? obj.user_id ?? obj.uuid,
          email: obj.email,
          role: obj.role,
        };
        setMe(nextMe);
      } catch (e: any) {
        if (e?.name === "AbortError") return;
        setBootError(e?.message || "Не удалось проверить сессию");
        clearAuth();
      } finally {
        setBootLoading(false);
      }
    };

    run();

    return () => {
      ac.abort();
      bootAbortRef.current = null;
    };
  }, [token, clearAuth]);

  useEffect(() => {
    const h = window.location.hash || "#/";
    if (h === "#/" || h === "#") {
      go(token ? "#/elections" : "#/login");
    }
  }, [token, go]);

  const onLogout = useCallback(async () => {
    try {
      if (token) {
        await apiRequest("/api/v1/auth/logout", { method: "POST", body: "{}" }, token);
      }
    } catch {
    } finally {
      clearAuth();
      go("#/login");
    }
  }, [token, clearAuth, go]);

  const Topbar = (
    <div style={styles.topbar}>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <h1 style={styles.title}>secure voting</h1>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        {bootLoading ? <span style={styles.muted}>Проверка сессии</span> : null}

        {authed ? (
          <>
            <span style={styles.muted}>
              {me?.email} <Badge text={String(me?.role || "unknown")} />
            </span>
            <button style={styles.btn} onClick={() => go("#/elections")}>
              Elections
            </button>
            {me?.role === "admin" ? (
              <button style={styles.btn} onClick={() => go("#/admin/create")}>
                Create
              </button>
            ) : null}
            <button style={styles.btnDanger} onClick={onLogout}>
              Logout
            </button>
          </>
        ) : (
          <button style={styles.btn} onClick={() => go("#/login")}>
            Login
          </button>
        )}
      </div>
    </div>
  );

  return (
    <div style={styles.page}>
      {Topbar}
      <ErrorBanner error={bootError} />

      {!authed && route.name !== "login" ? (
        <div style={styles.card}>
          <div style={{ margin: 0 }}>
            Требуется авторизация. Перейдите на <a href="#/login">#/login</a>.
          </div>
        </div>
      ) : null}

      {route.name === "login" ? (
        <LoginScreen
          token={token}
          onToken={(t) => {
            writeStoredToken(t);
            setToken(t);
            go("#/elections");
          }}
        />
      ) : null}

      {authed && route.name === "elections" ? <ElectionsList token={token} me={me} go={go} onUnauthorized={clearAuth} /> : null}
      {authed && route.name === "election" ? <ElectionView token={token} me={me} id={route.id} go={go} onUnauthorized={clearAuth} /> : null}
      {authed && route.name === "vote" ? <VoteView token={token} me={me} id={route.id} go={go} onUnauthorized={clearAuth} /> : null}
      {authed && route.name === "results" ? <ResultsView token={token} id={route.id} go={go} onUnauthorized={clearAuth} /> : null}
      {authed && route.name === "admin_create" ? <AdminCreateElection token={token} me={me} go={go} onUnauthorized={clearAuth} /> : null}

      {route.name === "not_found" ? (
        <div style={styles.card}>
          <b>404</b>
          <div style={styles.muted}>Unknown route: {route.raw}</div>
          <div style={{ marginTop: 8 }}>
            <button style={styles.btn} onClick={() => go("#/elections")}>
              Go elections
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function LoginScreen({ token, onToken }: { token: string | null; onToken: (t: string) => void }) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const [role, setRole] = useState<"admin" | "voter" | "researcher">("voter");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<unknown>(null);

  useEffect(() => {
    setErr(null);
    setRawResp(null);
  }, [mode]);

  const submit = async () => {
    setLoading(true);
    setErr(null);
    setRawResp(null);

    try {
      const e = email.trim();
      if (!e || !password) throw new Error("Введите email и пароль");

      if (mode === "register") {
        const resp = await apiRequest<any>(
          "/api/v1/auth/register",
          { method: "POST", body: JSON.stringify({ email: e, password, role }) },
          null
        );
        if (IS_DEV) setRawResp(resp);

        const t = extractToken(resp);
        if (!t) throw new Error("Регистрация выполнена, но токен не найден в ответе");
        onToken(t);
      } else {
        const resp = await apiRequest<any>(
          "/api/v1/auth/login",
          { method: "POST", body: JSON.stringify({ email: e, password }) },
          null
        );
        if (IS_DEV) setRawResp(resp);

        const t = extractToken(resp);
        if (!t) throw new Error("Вход выполнен, но токен не найден в ответе");
        onToken(t);
      }
    } catch (e: any) {
      setErr(e?.message || "Ошибка авторизации");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.grid2}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>{mode === "login" ? "Login" : "Register"}</h2>

        <div style={{ display: "flex", gap: 8, marginBottom: 12 }}>
          <button style={mode === "login" ? styles.btnPrimary : styles.btn} onClick={() => setMode("login")}>
            Login
          </button>
          <button style={mode === "register" ? styles.btnPrimary : styles.btn} onClick={() => setMode("register")}>
            Register
          </button>
        </div>

        <ErrorBanner error={err} />

        <label style={{ display: "block", marginBottom: 6 }}>Email</label>
        <input style={styles.input} value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />

        <div style={{ height: 10 }} />

        <label style={{ display: "block", marginBottom: 6 }}>Password</label>
        <input
          style={styles.input}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          type="password"
          autoComplete={mode === "login" ? "current-password" : "new-password"}
        />

        {mode === "register" ? (
          <>
            <div style={{ height: 10 }} />
            <label style={{ display: "block", marginBottom: 6 }}>Role</label>
            <select style={styles.input} value={role} onChange={(e) => setRole(e.target.value as any)}>
              <option value="voter">voter</option>
              <option value="researcher">researcher</option>
              <option value="admin">admin</option>
            </select>
          </>
        ) : null}

        <div style={{ height: 14 }} />

        <button style={styles.btnPrimary} onClick={submit} disabled={loading}>
          {loading ? "Loading" : mode === "login" ? "Login" : "Register"}
        </button>

        {token ? (
          <div style={{ marginTop: 12, ...styles.muted }}>
            В хранилище уже есть токен. Если вход не проходит, выполните Logout.
          </div>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Auth response</h3>
          {rawResp ? <JsonBlock value={rawResp} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Info</h3>
          <div style={styles.muted}>После авторизации можно перейти к списку выборов.</div>
        </div>
      )}
    </div>
  );
}

function ElectionsList({
  token,
  me,
  go,
  onUnauthorized,
}: {
  token: string | null;
  me: Me | null;
  go: (to: string) => void;
  onUnauthorized: () => void;
}) {
  const [items, setItems] = useState<ElectionSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const resp = await apiRequest<{ items: ElectionSummary[] }>(
        "/api/v1/elections",
        { method: "GET", signal: ac.signal },
        token
      );
      setItems(Array.isArray(resp.items) ? resp.items : []);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось загрузить список выборов");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [token, onUnauthorized]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  return (
    <div style={styles.card}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10 }}>
        <h2 style={{ margin: 0 }}>Elections</h2>
        <div style={{ display: "flex", gap: 8 }}>
          <button style={styles.btn} onClick={reload} disabled={loading}>
            Refresh
          </button>
          {me?.role === "admin" ? (
            <button style={styles.btnPrimary} onClick={() => go("#/admin/create")}>
              Create election
            </button>
          ) : null}
        </div>
      </div>

      <ErrorBanner error={err} />

      {loading ? <div style={{ marginTop: 10, ...styles.muted }}>Loading</div> : null}
      {!loading && items.length === 0 ? <div style={{ marginTop: 10, ...styles.muted }}>No elections</div> : null}

      <div style={{ marginTop: 12, display: "grid", gap: 10 }}>
        {items.map((e) => (
          <div key={e.id} style={{ ...styles.card, padding: 12 }}>
            <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
              <div>
                <div style={{ fontWeight: 700 }}>{e.title}</div>
                <div style={styles.muted}>{e.description || ""}</div>
              </div>
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Badge text={e.status} />
                <Badge text={e.access_mode} />
              </div>
            </div>

            <div style={{ marginTop: 8, display: "flex", gap: 10, flexWrap: "wrap", ...styles.muted, fontSize: 12 }}>
              <span>start: {e.start_at}</span>
              <span>end: {e.end_at}</span>
              {e.published_at ? <span>published: {e.published_at}</span> : null}
            </div>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <button style={styles.btnPrimary} onClick={() => go(`#/elections/${e.id}`)}>
                Open
              </button>
              <button style={styles.btn} onClick={() => go(`#/elections/${e.id}/vote`)}>
                Vote
              </button>
              <button style={styles.btn} onClick={() => go(`#/elections/${e.id}/results`)}>
                Results
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function ElectionView({
  token,
  me,
  id,
  go,
  onUnauthorized,
}: {
  token: string | null;
  me: Me | null;
  id: string;
  go: (to: string) => void;
  onUnauthorized: () => void;
}) {
  const [item, setItem] = useState<ElectionDetail | null>(null);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteCode, setInviteCode] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const isAdmin = me?.role === "admin";
  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);
    setInviteCode(null);

    try {
      const resp = await apiRequest<ElectionDetail>(`/api/v1/elections/${id}`, { method: "GET", signal: ac.signal }, token);
      setItem(resp);

      if (isAdmin) {
        try {
          const inv = await apiRequest<{ items: Invite[] }>(
            `/api/v1/elections/${id}/invites`,
            { method: "GET", signal: ac.signal },
            token
          );
          setInvites(Array.isArray(inv.items) ? inv.items : []);
        } catch {
          setInvites([]);
        }
      } else {
        setInvites([]);
      }
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось загрузить выборы");
      setItem(null);
      setInvites([]);
    } finally {
      setLoading(false);
    }
  }, [id, isAdmin, token, onUnauthorized]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  const doAction = async (action: string) => {
    setLoading(true);
    setErr(null);

    try {
      await apiRequest(`/api/v1/elections/${id}/actions/${action}`, { method: "POST", body: "{}" }, token);
      await reload();
    } catch (e: any) {
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Операция не выполнена");
    } finally {
      setLoading(false);
    }
  };

  const createInvite = async () => {
    const email = inviteEmail.trim();
    if (!email) {
      setErr("Введите email");
      return;
    }

    setLoading(true);
    setErr(null);
    setInviteCode(null);

    try {
      const resp = await apiRequest<InviteCreated>(
        `/api/v1/elections/${id}/invites`,
        { method: "POST", body: JSON.stringify({ email }) },
        token
      );
      setInviteCode(resp.invite_code || null);
      setInviteEmail("");
      await reload();
    } catch (e: any) {
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось создать приглашение");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Election</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => go("#/elections")}>
              Back
            </button>
            <button style={styles.btn} onClick={reload} disabled={loading}>
              Refresh
            </button>
            <button style={styles.btnPrimary} onClick={() => go(`#/elections/${id}/vote`)}>
              Vote
            </button>
            <button style={styles.btn} onClick={() => go(`#/elections/${id}/results`)}>
              Results
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Loading</div> : null}

        {item ? (
          <>
            <div style={{ marginTop: 8 }}>
              <div style={{ fontWeight: 800, fontSize: 18 }}>{item.title}</div>
              <div style={styles.muted}>{item.description || ""}</div>
            </div>

            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`status: ${item.status}`} />
              <Badge text={`access: ${item.access_mode}`} />
              <Badge text={`format: ${item.ballot_format}`} />
              <Badge text={`rule: ${item.tally_rule}`} />
            </div>

            <div style={{ marginTop: 10, ...styles.muted, fontSize: 12 }}>
              <div>start_at: {item.start_at}</div>
              <div>end_at: {item.end_at}</div>
              {item.publish_at ? <div>publish_at: {item.publish_at}</div> : null}
              {item.published_at ? <div>published_at: {item.published_at}</div> : null}
              <div>committee_size: {item.committee_size ?? 1}</div>
              <div>quota_type: {item.quota_type ?? "null"}</div>
              <div>show_aggregates: {String(item.show_aggregates)}</div>
            </div>

            <hr style={styles.hr} />

            <h3 style={{ margin: 0 }}>Candidates</h3>
            <div style={{ marginTop: 8, display: "grid", gap: 6 }}>
              {item.candidates?.map((c) => (
                <div key={c.id} style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                  <div>
                    <b>{c.name}</b>
                    <div style={styles.muted}>{c.id}</div>
                  </div>
                </div>
              ))}
            </div>

            {isAdmin ? (
              <>
                <hr style={styles.hr} />
                <h3 style={{ margin: 0 }}>Admin actions</h3>
                <div style={{ marginTop: 8, display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <button style={styles.btn} onClick={() => doAction("schedule")} disabled={loading}>
                    schedule
                  </button>
                  <button style={styles.btn} onClick={() => doAction("open")} disabled={loading}>
                    open
                  </button>
                  <button style={styles.btn} onClick={() => doAction("pause")} disabled={loading}>
                    pause
                  </button>
                  <button style={styles.btn} onClick={() => doAction("resume")} disabled={loading}>
                    resume
                  </button>
                  <button style={styles.btnDanger} onClick={() => doAction("close")} disabled={loading}>
                    close
                  </button>
                  <button style={styles.btnPrimary} onClick={() => doAction("publish")} disabled={loading}>
                    publish
                  </button>
                </div>

                {item.access_mode === "invite" ? (
                  <>
                    <hr style={styles.hr} />
                    <h3 style={{ margin: 0 }}>Invites</h3>

                    <div style={{ marginTop: 8, display: "flex", gap: 8, alignItems: "center" }}>
                      <input
                        style={styles.input}
                        value={inviteEmail}
                        onChange={(e) => setInviteEmail(e.target.value)}
                        placeholder="email"
                      />
                      <button style={styles.btnPrimary} onClick={createInvite} disabled={loading}>
                        Create invite
                      </button>
                    </div>

                    {inviteCode ? (
                      <div style={{ marginTop: 10, ...styles.card, borderColor: "#bbf7d0", background: "#f0fdf4" }}>
                        <div style={{ fontWeight: 700 }}>Invite code</div>
                        <div style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{inviteCode}</div>
                      </div>
                    ) : null}

                    <div style={{ marginTop: 10, display: "grid", gap: 8 }}>
                      {invites.map((it) => (
                        <div key={it.id} style={{ ...styles.card, padding: 10 }}>
                          <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                            <div>
                              <b>{it.email}</b>
                              <div style={styles.muted}>{it.id}</div>
                            </div>
                            <Badge text={it.status} />
                          </div>
                          <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
                            <div>created_at: {it.created_at}</div>
                            {it.sent_at ? <div>sent_at: {it.sent_at}</div> : null}
                            {it.accepted_at ? <div>accepted_at: {it.accepted_at}</div> : null}
                          </div>
                        </div>
                      ))}
                      {invites.length === 0 ? <div style={styles.muted}>No invites</div> : null}
                    </div>
                  </>
                ) : null}
              </>
            ) : null}
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Election JSON</h3>
          {item ? <JsonBlock value={item} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}

function VoteView({
  token,
  id,
  go,
  onUnauthorized,
}: {
  token: string | null;
  me: Me | null;
  id: string;
  go: (to: string) => void;
  onUnauthorized: () => void;
}) {
  const [meta, setMeta] = useState<BallotMeta | null>(null);
  const [my, setMy] = useState<MyBallotResp | null>(null);

  const [approvalSet, setApprovalSet] = useState<string[]>([]);
  const [ranking, setRanking] = useState<string[]>([]);
  const [scores, setScores] = useState<Record<string, number>>({});

  const [idemKey, setIdemKey] = useState<string>(() => newIdempotencyKey());

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [submitResp, setSubmitResp] = useState<unknown>(null);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);
    setSubmitResp(null);

    try {
      const m = await apiRequest<BallotMeta>(`/api/v1/elections/${id}/ballot`, { method: "GET", signal: ac.signal }, token);
      setMeta(m);

      const mb = await apiRequest<MyBallotResp>(`/api/v1/elections/${id}/ballots/me`, { method: "GET", signal: ac.signal }, token);
      setMy(mb);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось загрузить бюллетень");
      setMeta(null);
      setMy(null);
    } finally {
      setLoading(false);
    }
  }, [id, token, onUnauthorized]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  useEffect(() => {
    if (!meta) return;
    if (meta.ballot_format !== "score") return;
    if (meta.score_min == null) return;

    setScores((prev) => {
      if (Object.keys(prev).length > 0) return prev;
      const next: Record<string, number> = {};
      for (const c of meta.candidates) next[c.id] = meta.score_min as number;
      return next;
    });
  }, [meta]);

  const toggleApproval = (cid: string) => {
    if (!meta) return;
    const max = meta.approval_max_choices ?? null;

    setApprovalSet((prev) => {
      const has = prev.includes(cid);
      if (has) return prev.filter((x) => x !== cid);
      if (max != null && max > 0 && prev.length >= max) return prev;
      return [...prev, cid];
    });
  };

  const toggleRank = (cid: string) => {
    if (!meta) return;
    const topK = meta.ranking_top_k ?? null;

    setRanking((prev) => {
      const has = prev.includes(cid);
      if (has) return prev.filter((x) => x !== cid);
      if (topK != null && topK > 0 && prev.length >= topK) return prev;
      return [...prev, cid];
    });
  };

  const setScore = (cid: string, v: number) => {
    setScores((prev) => ({ ...prev, [cid]: v }));
  };

  const validateBeforeSubmit = (): string | null => {
    if (!meta) return "Нет метаданных бюллетеня";

    if (meta.ballot_format === "approval") {
      if (approvalSet.length === 0) return "Выберите хотя бы одного кандидата";
      const max = meta.approval_max_choices ?? null;
      if (max != null && max > 0 && approvalSet.length > max) return "Превышен лимит выбора";
      return null;
    }

    if (meta.ballot_format === "ranking") {
      if (ranking.length === 0) return "Сформируйте ранжирование";
      const topK = meta.ranking_top_k ?? null;
      if (topK != null && topK > 0 && ranking.length > topK) return "Превышен topK";
      const uniq = new Set(ranking);
      if (uniq.size !== ranking.length) return "В ранжировании есть повторы";
      return null;
    }

    if (meta.ballot_format === "score") {
      const min = meta.score_min;
      const max = meta.score_max;
      const step = meta.score_step;

      if (min == null || max == null || step == null || step <= 0) return "Некорректные параметры оценки";
      for (const c of meta.candidates) {
        const v = scores[c.id];
        if (v === undefined || v === null) {
          if (meta.score_allow_skip) continue;
          return "Заполните все оценки";
        }
        if (!Number.isFinite(v)) return "Некорректное значение оценки";
        if (v < min || v > max) return "Оценка вне диапазона";
        if (((v - min) % step) !== 0) return "Оценка не соответствует шагу";
      }
      return null;
    }

    return "Неизвестный формат бюллетеня";
  };

  const submit = async () => {
    const v = validateBeforeSubmit();
    if (v) {
      setErr(v);
      return;
    }
    if (!meta) return;

    setLoading(true);
    setErr(null);
    setSubmitResp(null);

    try {
      const body: AnyObj = {};

      if (meta.ballot_format === "approval") body.approval_set = approvalSet;
      if (meta.ballot_format === "ranking") body.ranking = ranking;
      if (meta.ballot_format === "score") {
        const out: Record<string, number> = {};
        for (const c of meta.candidates) {
          const val = scores[c.id];
          if (val === undefined || val === null) continue;
          out[c.id] = val;
        }
        body.scores = out;
      }

      const resp = await apiRequest<any>(
        `/api/v1/elections/${id}/ballots/submit`,
        { method: "POST", headers: { "Idempotency-Key": idemKey }, body: JSON.stringify(body) },
        token
      );

      if (IS_DEV) setSubmitResp(resp);

      setIdemKey(newIdempotencyKey());
      await reload();
    } catch (e: any) {
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось отправить бюллетень");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Vote</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => go(`#/elections/${id}`)}>
              Election
            </button>
            <button style={styles.btn} onClick={() => go("#/elections")}>
              Back
            </button>
            <button style={styles.btn} onClick={reload} disabled={loading}>
              Refresh
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Loading</div> : null}

        {my ? (
          <div style={{ marginTop: 10, display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
            <Badge text={`my ballot: ${my.status}`} />
            {my.submitted_at ? <span style={styles.muted}>submitted_at: {my.submitted_at}</span> : null}
            {my.updated_at ? <span style={styles.muted}>updated_at: {my.updated_at}</span> : null}
          </div>
        ) : null}

        {meta ? (
          <>
            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`format: ${meta.ballot_format}`} />
              <Badge text={`rule: ${meta.tally_rule}`} />
              {meta.ballot_format === "approval" && meta.approval_max_choices != null ? <Badge text={`max: ${meta.approval_max_choices}`} /> : null}
              {meta.ballot_format === "ranking" && meta.ranking_top_k != null ? <Badge text={`topK: ${meta.ranking_top_k}`} /> : null}
              {meta.ballot_format === "score" ? (
                <Badge text={`score: ${meta.score_min ?? "?"}..${meta.score_max ?? "?"} step ${meta.score_step ?? "?"} skip ${String(meta.score_allow_skip)}`} />
              ) : null}
            </div>

            <hr style={styles.hr} />

            {meta.ballot_format === "approval" ? (
              <>
                <h3 style={{ marginTop: 0 }}>Approval</h3>
                <div style={styles.muted}>Выберите кандидатов</div>
                <div style={{ marginTop: 10, display: "grid", gap: 8 }}>
                  {meta.candidates.map((c) => {
                    const checked = approvalSet.includes(c.id);
                    return (
                      <label
                        key={c.id}
                        style={{
                          display: "flex",
                          gap: 10,
                          alignItems: "center",
                          padding: 10,
                          border: "1px solid #e5e7eb",
                          borderRadius: 12,
                          cursor: "pointer",
                          userSelect: "none",
                        }}
                      >
                        <input type="checkbox" checked={checked} onChange={() => toggleApproval(c.id)} />
                        <div style={{ flex: 1 }}>
                          <b>{c.name}</b>
                          <div style={styles.muted}>{c.id}</div>
                        </div>
                      </label>
                    );
                  })}
                </div>
              </>
            ) : null}

            {meta.ballot_format === "ranking" ? (
              <>
                <h3 style={{ marginTop: 0 }}>Ranking</h3>
                <div style={styles.muted}>Нажмите кандидата, чтобы добавить или удалить</div>

                <div style={{ marginTop: 10, ...styles.card, background: "#f9fafb" }}>
                  <div style={{ fontWeight: 700 }}>Текущее ранжирование</div>
                  {ranking.length === 0 ? (
                    <div style={styles.muted}>Empty</div>
                  ) : (
                    <ol style={{ margin: "6px 0 0 18px" }}>
                      {ranking.map((cid) => {
                        const c = meta.candidates.find((x) => x.id === cid);
                        return (
                          <li key={cid}>
                            {c ? c.name : cid} <span style={{ ...styles.muted, fontSize: 12 }}>{cid}</span>
                          </li>
                        );
                      })}
                    </ol>
                  )}
                </div>

                <div style={{ marginTop: 10, display: "grid", gap: 8 }}>
                  {meta.candidates.map((c) => {
                    const selected = ranking.includes(c.id);
                    return (
                      <button
                        key={c.id}
                        style={{
                          ...styles.btn,
                          textAlign: "left",
                          padding: 12,
                          borderRadius: 12,
                          borderColor: selected ? "#111827" : "#e5e7eb",
                        }}
                        onClick={() => toggleRank(c.id)}
                      >
                        <div style={{ display: "flex", justifyContent: "space-between", gap: 10 }}>
                          <div>
                            <b>{c.name}</b>
                            <div style={styles.muted}>{c.id}</div>
                          </div>
                          {selected ? <Badge text={`rank ${ranking.indexOf(c.id) + 1}`} /> : <span />}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </>
            ) : null}

            {meta.ballot_format === "score" ? (
              <>
                <h3 style={{ marginTop: 0 }}>Score</h3>
                <div style={styles.muted}>Заполните оценки</div>

                <div style={{ marginTop: 10, display: "grid", gap: 10 }}>
                  {meta.candidates.map((c) => {
                    const v = scores[c.id];
                    const min = meta.score_min ?? 0;
                    const max = meta.score_max ?? 10;
                    const step = meta.score_step ?? 1;

                    const missing = !meta.score_allow_skip && (v === undefined || v === null);
                    return (
                      <div
                        key={c.id}
                        style={{
                          ...styles.card,
                          padding: 12,
                          borderColor: missing ? "#fecaca" : "#e5e7eb",
                          background: missing ? "#fff1f2" : "white",
                        }}
                      >
                        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
                          <div>
                            <b>{c.name}</b>
                            <div style={styles.muted}>{c.id}</div>
                          </div>

                          <div style={{ width: 220 }}>
                            <input
                              style={styles.input}
                              type="number"
                              min={min}
                              max={max}
                              step={step}
                              value={v ?? ""}
                              onChange={(e) => {
                                const raw = e.target.value;
                                if (raw.trim() === "") {
                                  if (meta.score_allow_skip) {
                                    setScores((prev) => {
                                      const next = { ...prev };
                                      delete next[c.id];
                                      return next;
                                    });
                                  }
                                  return;
                                }
                                const num = Number(raw);
                                if (Number.isFinite(num)) setScore(c.id, num);
                              }}
                            />
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </>
            ) : null}

            <hr style={styles.hr} />

            <div style={{ display: "grid", gap: 8 }}>
              <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
                <b>Idempotency-Key</b>
                <span style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{idemKey}</span>
                <button style={styles.btn} onClick={() => setIdemKey(newIdempotencyKey())} disabled={loading}>
                  Regen
                </button>
              </div>

              <button style={styles.btnPrimary} onClick={submit} disabled={loading}>
                {loading ? "Submitting" : "Submit ballot"}
              </button>
            </div>

            {IS_DEV && submitResp ? (
              <>
                <hr style={styles.hr} />
                <h3 style={{ marginTop: 0 }}>Submit response</h3>
                <JsonBlock value={submitResp} />
              </>
            ) : null}
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Debug</h3>
          <div style={{ display: "grid", gap: 10 }}>
            <div>
              <div style={styles.muted}>Ballot meta</div>
              {meta ? <JsonBlock value={meta} /> : <div style={styles.muted}>Empty</div>}
            </div>
            <div>
              <div style={styles.muted}>My ballot</div>
              {my ? <JsonBlock value={my} /> : <div style={styles.muted}>Empty</div>}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function ResultsView({
  token,
  id,
  go,
  onUnauthorized,
}: {
  token: string | null;
  id: string;
  go: (to: string) => void;
  onUnauthorized: () => void;
}) {
  const [res, setRes] = useState<ResultResp | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const abortRef = useRef<AbortController | null>(null);

  const reload = useCallback(async () => {
    abortRef.current?.abort();
    const ac = new AbortController();
    abortRef.current = ac;

    setLoading(true);
    setErr(null);

    try {
      const r = await apiRequest<ResultResp>(`/api/v1/elections/${id}/results`, { method: "GET", signal: ac.signal }, token);
      setRes(r);
    } catch (e: any) {
      if (e?.name === "AbortError") return;
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось загрузить результаты");
      setRes(null);
    } finally {
      setLoading(false);
    }
  }, [id, token, onUnauthorized]);

  useEffect(() => {
    reload();
    return () => abortRef.current?.abort();
  }, [reload]);

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Results</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => go(`#/elections/${id}`)}>
              Election
            </button>
            <button style={styles.btn} onClick={() => go("#/elections")}>
              Back
            </button>
            <button style={styles.btn} onClick={reload} disabled={loading}>
              Refresh
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />
        {loading ? <div style={styles.muted}>Loading</div> : null}

        {res ? (
          <>
            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`method: ${res.method}`} />
              <Badge text={`version: ${String(res.version)}`} />
              <Badge text={`published_at: ${res.published_at ?? "null"}`} />
            </div>

            <hr style={styles.hr} />

            <h3 style={{ marginTop: 0 }}>Winners</h3>
            <JsonBlock value={res.winners} />

            {res.metrics != null ? (
              <>
                <h3>Metrics</h3>
                <JsonBlock value={res.metrics} />
              </>
            ) : null}

            {res.protocol != null ? (
              <>
                <h3>Protocol</h3>
                <JsonBlock value={res.protocol} />
              </>
            ) : null}

            {res.params != null ? (
              <>
                <h3>Params</h3>
                <JsonBlock value={res.params} />
              </>
            ) : null}
          </>
        ) : null}
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Results JSON</h3>
          {res ? <JsonBlock value={res} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}

function AdminCreateElection({
  token,
  me,
  go,
  onUnauthorized,
}: {
  token: string | null;
  me: Me | null;
  go: (to: string) => void;
  onUnauthorized: () => void;
}) {
  const [title, setTitle] = useState("Election");
  const [description, setDescription] = useState("");

  const [ballotFormat, setBallotFormat] = useState<"approval" | "ranking" | "score">("ranking");
  const [tallyRule, setTallyRule] = useState("plurality");

  const [committeeSize, setCommitteeSize] = useState<number>(1);
  const [quotaType, setQuotaType] = useState<"hare" | "droop">("hare");

  const [accessMode, setAccessMode] = useState<"open" | "invite">("open");
  const [showAggregates, setShowAggregates] = useState(true);

  const [startAt, setStartAt] = useState(nowRfc3339Plus(0));
  const [endAt, setEndAt] = useState(nowRfc3339Plus(60));

  const [approvalMax, setApprovalMax] = useState<number>(2);
  const [rankingTopK, setRankingTopK] = useState<number>(3);

  const [scoreMin, setScoreMin] = useState<number>(0);
  const [scoreMax, setScoreMax] = useState<number>(10);
  const [scoreStep, setScoreStep] = useState<number>(1);
  const [scoreAllowSkip, setScoreAllowSkip] = useState<boolean>(false);

  const [candidatesText, setCandidatesText] = useState("Alice\nBob\nCarol");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [createdID, setCreatedID] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<unknown>(null);

  useEffect(() => {
    if (ballotFormat === "approval") {
      if (tallyRule !== "approval") setTallyRule("approval");
    } else if (ballotFormat === "ranking") {
      if (tallyRule === "approval") setTallyRule("plurality");
    } else if (ballotFormat === "score") {
      if (tallyRule !== "plurality" && tallyRule !== "borda") setTallyRule("plurality");
    }
  }, [ballotFormat]);

  const submit = async () => {
    if (me?.role !== "admin") {
      setErr("Только admin может создавать выборы");
      return;
    }

    setLoading(true);
    setErr(null);
    setCreatedID(null);
    setRawResp(null);

    try {
      const names = candidatesText
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean);

      const uniq: string[] = [];
      const seen = new Set<string>();
      for (const n of names) {
        const key = n.toLowerCase();
        if (seen.has(key)) continue;
        seen.add(key);
        uniq.push(n);
      }

      if (uniq.length < 2) throw new Error("Добавьте минимум двух кандидатов");

      const candidates = uniq.map((name) => ({ name }));

      const body: AnyObj = {
        title: title.trim(),
        description: description.trim() ? description.trim() : null,
        start_at: startAt.trim(),
        end_at: endAt.trim(),
        tally_rule: tallyRule.trim(),
        ballot_format: ballotFormat,
        committee_size: committeeSize,
        quota_type: committeeSize > 1 ? quotaType : null,
        access_mode: accessMode,
        publish_at: null,
        show_aggregates: showAggregates,
        candidates,
      };

      if (ballotFormat === "approval") {
        body.approval_max_choices = approvalMax;
      } else if (ballotFormat === "ranking") {
        body.ranking_top_k = rankingTopK;
      } else if (ballotFormat === "score") {
        body.score_min = scoreMin;
        body.score_max = scoreMax;
        body.score_step = scoreStep;
        body.score_allow_skip = scoreAllowSkip;
      }

      const resp = await apiRequest<any>("/api/v1/elections", { method: "POST", body: JSON.stringify(body) }, token);
      if (IS_DEV) setRawResp(resp);

      const eid = resp?.id;
      if (typeof eid === "string" && eid.trim()) setCreatedID(eid.trim());
      else throw new Error("Выборы созданы, но id не найден в ответе");
    } catch (e: any) {
      if (e?.status === 401) onUnauthorized();
      setErr(e?.message || "Не удалось создать выборы");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Create election</h2>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <button style={styles.btn} onClick={() => go("#/elections")}>
              Back
            </button>
          </div>
        </div>

        <ErrorBanner error={err} />

        <div style={styles.grid2}>
          <div>
            <label>Title</label>
            <input style={styles.input} value={title} onChange={(e) => setTitle(e.target.value)} />
          </div>
          <div>
            <label>Description</label>
            <input style={styles.input} value={description} onChange={(e) => setDescription(e.target.value)} />
          </div>

          <div>
            <label>StartAt (RFC3339)</label>
            <input style={styles.input} value={startAt} onChange={(e) => setStartAt(e.target.value)} />
          </div>
          <div>
            <label>EndAt (RFC3339)</label>
            <input style={styles.input} value={endAt} onChange={(e) => setEndAt(e.target.value)} />
          </div>

          <div>
            <label>Ballot format</label>
            <select style={styles.input} value={ballotFormat} onChange={(e) => setBallotFormat(e.target.value as any)}>
              <option value="ranking">ranking</option>
              <option value="approval">approval</option>
              <option value="score">score</option>
            </select>
          </div>

          <div>
            <label>Tally rule</label>
            <select style={styles.input} value={tallyRule} onChange={(e) => setTallyRule(e.target.value)}>
              <option value="plurality">plurality</option>
              <option value="borda">borda</option>
              <option value="approval">approval</option>
            </select>
          </div>

          <div>
            <label>Committee size</label>
            <input style={styles.input} type="number" min={1} value={committeeSize} onChange={(e) => setCommitteeSize(Number(e.target.value))} />
          </div>
          <div>
            <label>Quota type</label>
            <select style={styles.input} value={quotaType} onChange={(e) => setQuotaType(e.target.value as any)}>
              <option value="hare">hare</option>
              <option value="droop">droop</option>
            </select>
          </div>

          <div>
            <label>Access mode</label>
            <select style={styles.input} value={accessMode} onChange={(e) => setAccessMode(e.target.value as any)}>
              <option value="open">open</option>
              <option value="invite">invite</option>
            </select>
          </div>

          <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
            <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <input type="checkbox" checked={showAggregates} onChange={(e) => setShowAggregates(e.target.checked)} />
              show_aggregates
            </label>
          </div>
        </div>

        <hr style={styles.hr} />

        {ballotFormat === "approval" ? (
          <div style={styles.grid2}>
            <div>
              <label>approval_max_choices</label>
              <input style={styles.input} type="number" min={1} value={approvalMax} onChange={(e) => setApprovalMax(Number(e.target.value))} />
            </div>
            <div />
          </div>
        ) : null}

        {ballotFormat === "ranking" ? (
          <div style={styles.grid2}>
            <div>
              <label>ranking_top_k</label>
              <input style={styles.input} type="number" min={1} value={rankingTopK} onChange={(e) => setRankingTopK(Number(e.target.value))} />
            </div>
            <div />
          </div>
        ) : null}

        {ballotFormat === "score" ? (
          <div style={styles.grid2}>
            <div>
              <label>score_min</label>
              <input style={styles.input} type="number" value={scoreMin} onChange={(e) => setScoreMin(Number(e.target.value))} />
            </div>
            <div>
              <label>score_max</label>
              <input style={styles.input} type="number" value={scoreMax} onChange={(e) => setScoreMax(Number(e.target.value))} />
            </div>
            <div>
              <label>score_step</label>
              <input style={styles.input} type="number" min={1} value={scoreStep} onChange={(e) => setScoreStep(Number(e.target.value))} />
            </div>
            <div style={{ display: "flex", alignItems: "center" }}>
              <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <input type="checkbox" checked={scoreAllowSkip} onChange={(e) => setScoreAllowSkip(e.target.checked)} />
                score_allow_skip
              </label>
            </div>
          </div>
        ) : null}

        <hr style={styles.hr} />

        <div>
          <label>Candidates (one per line)</label>
          <textarea
            style={{ ...styles.input, minHeight: 120, fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}
            value={candidatesText}
            onChange={(e) => setCandidatesText(e.target.value)}
          />
        </div>

        <div style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
          <button style={styles.btnPrimary} onClick={submit} disabled={loading}>
            {loading ? "Creating" : "Create"}
          </button>
          {createdID ? (
            <button style={styles.btn} onClick={() => go(`#/elections/${createdID}`)}>
              Open
            </button>
          ) : null}
        </div>
      </div>

      {IS_DEV ? (
        <div style={styles.card}>
          <h3 style={{ marginTop: 0 }}>Create response</h3>
          {rawResp ? <JsonBlock value={rawResp} /> : <div style={styles.muted}>Empty</div>}
        </div>
      ) : null}
    </div>
  );
}
