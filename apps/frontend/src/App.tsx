import React, { useEffect, useMemo, useState } from "react";

type AnyObj = Record<string, any>;

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
  params?: any;
  winners: any;
  metrics?: any;
  protocol?: any;
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

function nowRfc3339Plus(minutes: number) {
  const d = new Date(Date.now() + minutes * 60_000);
  return d.toISOString();
}

function safeJsonParse(text: string): any | null {
  const t = text.trim();
  if (!t) return null;
  try {
    return JSON.parse(t);
  } catch {
    return null;
  }
}

function pickToken(obj: any): string | null {
  if (!obj || typeof obj !== "object") return null;
  const candidates = [
    obj.access_token,
    obj.token,
    obj.accessToken,
    obj.jwt,
    obj.data?.access_token,
    obj.data?.token,
  ];
  for (const c of candidates) {
    if (typeof c === "string" && c.trim()) return c.trim();
  }
  return null;
}

function newIdemKey(): string {
  const g = (globalThis as any);
  const uuid =
    typeof g?.crypto?.randomUUID === "function"
      ? g.crypto.randomUUID()
      : `r${Math.random().toString(16).slice(2)}${Date.now().toString(16)}`;
  // допустимые символы по validateIdempotencyKey: буквы/цифры/-_:.@
  return `idem-${uuid}`;
}

function parseApiError(data: any, fallback: string): string {
  if (!data) return fallback;
  if (typeof data === "string") return data;

  const direct =
    data.message ||
    data.error ||
    data.code ||
    data.detail ||
    data.reason ||
    data?.error?.message ||
    data?.error?.code ||
    data?.error?.error ||
    data?.error?.detail;

  if (typeof direct === "string" && direct.trim()) return direct.trim();

  try {
    return JSON.stringify(data);
  } catch {
    return fallback;
  }
}

async function apiRequest<T>(
  path: string,
  opts: RequestInit,
  token: string | null
): Promise<T> {
  const headers = new Headers(opts.headers || {});
  headers.set("Accept", "application/json");
  if (opts.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (token) headers.set("Authorization", `Bearer ${token}`);

  const res = await fetch(path, { ...opts, headers });
  const text = await res.text();
  const data = safeJsonParse(text);

  if (!res.ok) {
    const msg = parseApiError(
      data,
      `HTTP ${res.status} ${res.statusText || "error"}`
    );
    throw new Error(msg);
  }

  return (data ?? ({} as any)) as T;
}

function useHashRoute() {
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
    if (parts[0] === "elections" && parts.length === 1)
      return { name: "elections" as const };
    if (parts[0] === "elections" && parts.length === 2)
      return { name: "election" as const, id: parts[1] };
    if (parts[0] === "elections" && parts.length === 3 && parts[2] === "vote")
      return { name: "vote" as const, id: parts[1] };
    if (
      parts[0] === "elections" &&
      parts.length === 3 &&
      parts[2] === "results"
    )
      return { name: "results" as const, id: parts[1] };
    if (parts[0] === "admin" && parts[1] === "create")
      return { name: "admin_create" as const };

    return { name: "not_found" as const, raw };
  }, [hash]);

  const go = (to: string) => {
    if (!to.startsWith("#")) window.location.hash = `#${to}`;
    else window.location.hash = to;
  };

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

function JsonBlock({ value }: { value: any }) {
  return <pre style={styles.pre}>{JSON.stringify(value, null, 2)}</pre>;
}

export default function App() {
  const { route, go } = useHashRoute();

  const [token, setToken] = useState<string | null>(() => {
    const t = localStorage.getItem("sv_token");
    return t && t.trim() ? t.trim() : null;
  });

  const [me, setMe] = useState<Me | null>(null);
  const [bootError, setBootError] = useState<string | null>(null);
  const [bootLoading, setBootLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function boot() {
      if (!token) {
        setMe(null);
        return;
      }
      setBootLoading(true);
      setBootError(null);
      try {
        const resp = await apiRequest<any>("/api/v1/auth/me", { method: "GET" }, token);
        if (cancelled) return;

        const obj = resp?.user ?? resp?.data?.user ?? resp?.data ?? resp ?? {};
        const nextMe: Me = {
          id: obj.id ?? obj.user_id ?? obj.uuid,
          email: obj.email,
          role: obj.role,
        };
        setMe(nextMe);
      } catch (e: any) {
        if (cancelled) return;
        setBootError(e?.message || "auth/me failed");
        setMe(null);
        localStorage.removeItem("sv_token");
        setToken(null);
      } finally {
        if (!cancelled) setBootLoading(false);
      }
    }

    boot();
    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!window.location.hash || window.location.hash === "#/") {
      if (token) go("#/elections");
      else go("#/login");
    }
  }, [token, go]);

  const authed = !!token && !!me?.email;

  const onLogout = async () => {
    try {
      if (token) {
        await apiRequest("/api/v1/auth/logout", { method: "POST", body: "{}" }, token);
      }
    } catch {
    }
    localStorage.removeItem("sv_token");
    setToken(null);
    setMe(null);
    go("#/login");
  };

  const Topbar = (
    <div style={styles.topbar}>
      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        <h1 style={styles.title}>secure voting</h1>
        <span style={styles.muted}>demo UI</span>
      </div>

      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
        {bootLoading ? <span style={styles.muted}>checking session…</span> : null}
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
          <p style={{ margin: 0 }}>
            Ты не авторизован. Перейди в <a href="#/login">#/login</a>.
          </p>
        </div>
      ) : null}

      {route.name === "login" ? (
        <LoginScreen
          token={token}
          onToken={(t) => {
            localStorage.setItem("sv_token", t);
            setToken(t);
            go("#/elections");
          }}
        />
      ) : null}

      {authed && route.name === "elections" ? (
        <ElectionsList token={token} me={me} go={go} />
      ) : null}

      {authed && route.name === "election" ? (
        <ElectionView token={token} me={me} id={route.id} go={go} />
      ) : null}

      {authed && route.name === "vote" ? (
        <VoteView token={token} me={me} id={route.id} go={go} />
      ) : null}

      {authed && route.name === "results" ? (
        <ResultsView token={token} me={me} id={route.id} go={go} />
      ) : null}

      {authed && route.name === "admin_create" ? (
        <AdminCreateElection token={token} me={me} go={go} />
      ) : null}

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

function LoginScreen({
  token,
  onToken,
}: {
  token: string | null;
  onToken: (t: string) => void;
}) {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const [role, setRole] = useState<"admin" | "voter" | "researcher">("voter");

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [rawResp, setRawResp] = useState<any>(null);

  useEffect(() => {
    setErr(null);
    setRawResp(null);
  }, [mode]);

  const submit = async () => {
    setLoading(true);
    setErr(null);
    setRawResp(null);
    try {
      if (!email.trim() || !password) throw new Error("email/password required");

      if (mode === "register") {
        const resp = await apiRequest<any>(
          "/api/v1/auth/register",
          {
            method: "POST",
            body: JSON.stringify({ email: email.trim(), password, role }),
          },
          null
        );
        setRawResp(resp);

        const t = pickToken(resp);
        if (t) onToken(t);
      } else {
        const resp = await apiRequest<any>(
          "/api/v1/auth/login",
          {
            method: "POST",
            body: JSON.stringify({ email: email.trim(), password }),
          },
          null
        );
        setRawResp(resp);

        const t = pickToken(resp);
        if (!t) {
          throw new Error(
            "login ok, но не нашёл токен в ответе (ожидал access_token/token/accessToken). Проверь формат ответа backend."
          );
        }
        onToken(t);
      }
    } catch (e: any) {
      setErr(e?.message || "auth failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={styles.grid2}>
      <div style={styles.card}>
        <h2 style={{ marginTop: 0 }}>{mode === "login" ? "Login" : "Register"}</h2>

        <div style={{ display: "flex", gap: 8, marginBottom: 12 }}>
          <button
            style={mode === "login" ? styles.btnPrimary : styles.btn}
            onClick={() => setMode("login")}
          >
            Login
          </button>
          <button
            style={mode === "register" ? styles.btnPrimary : styles.btn}
            onClick={() => setMode("register")}
          >
            Register
          </button>
        </div>

        <ErrorBanner error={err} />

        <label style={{ display: "block", marginBottom: 6 }}>Email</label>
        <input
          style={styles.input}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="user@example.com"
        />

        <div style={{ height: 10 }} />

        <label style={{ display: "block", marginBottom: 6 }}>Password</label>
        <input
          style={styles.input}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          type="password"
          placeholder="password"
        />

        {mode === "register" ? (
          <>
            <div style={{ height: 10 }} />
            <label style={{ display: "block", marginBottom: 6 }}>Role</label>
            <select
              style={styles.input}
              value={role}
              onChange={(e) => setRole(e.target.value as any)}
            >
              <option value="voter">voter</option>
              <option value="researcher">researcher</option>
              <option value="admin">admin</option>
            </select>
            <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
              Для демо удобно создать отдельного admin.
            </div>
          </>
        ) : null}

        <div style={{ height: 14 }} />

        <button style={styles.btnPrimary} onClick={submit} disabled={loading}>
          {loading ? "…" : mode === "login" ? "Login" : "Register"}
        </button>

        {token ? (
          <>
            <div style={{ height: 12 }} />
            <div style={styles.muted}>
              Сейчас есть токен в localStorage. Если что-то сломалось — сделайте Logout.
            </div>
          </>
        ) : null}
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Debug: raw auth response</h3>
        {rawResp ? <JsonBlock value={rawResp} /> : <div style={styles.muted}>—</div>}
        <div style={{ marginTop: 10, ...styles.muted, fontSize: 12 }}>
          Если login/register не отдаёт токен, покажите сюда реальный JSON, и я подстрою парсер.
        </div>
      </div>
    </div>
  );
}

function ElectionsList({ token, me, go }: { token: string | null; me: Me | null; go: (to: string) => void }) {
  const [items, setItems] = useState<ElectionSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const reload = async () => {
    setLoading(true);
    setErr(null);
    try {
      const resp = await apiRequest<{ items: ElectionSummary[] }>(
        "/api/v1/elections",
        { method: "GET" },
        token
      );
      setItems(resp.items || []);
    } catch (e: any) {
      setErr(e?.message || "list elections failed");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, []);

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

      {loading ? <div style={{ marginTop: 10, ...styles.muted }}>Loading…</div> : null}
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

function ElectionView({ token, me, id, go }: { token: string | null; me: Me | null; id: string; go: (to: string) => void }) {
  const [item, setItem] = useState<ElectionDetail | null>(null);
  const [invites, setInvites] = useState<Invite[]>([]);
  const [inviteEmail, setInviteEmail] = useState("");
  const [inviteCode, setInviteCode] = useState<string | null>(null);

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const isAdmin = me?.role === "admin";

  const reload = async () => {
    setLoading(true);
    setErr(null);
    setInviteCode(null);
    try {
      const resp = await apiRequest<ElectionDetail>(`/api/v1/elections/${id}`, { method: "GET" }, token);
      setItem(resp);

      if (isAdmin) {
        try {
          const inv = await apiRequest<{ items: Invite[] }>(
            `/api/v1/elections/${id}/invites`,
            { method: "GET" },
            token
          );
          setInvites(inv.items || []);
        } catch {
          setInvites([]);
        }
      } else {
        setInvites([]);
      }
    } catch (e: any) {
      setErr(e?.message || "get election failed");
      setItem(null);
      setInvites([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, [id]);

  const doAction = async (action: string) => {
    setLoading(true);
    setErr(null);
    try {
      await apiRequest(`/api/v1/elections/${id}/actions/${action}`, { method: "POST", body: "{}" }, token);
      await reload();
    } catch (e: any) {
      setErr(e?.message || "action failed");
    } finally {
      setLoading(false);
    }
  };

  const createInvite = async () => {
    setLoading(true);
    setErr(null);
    setInviteCode(null);
    try {
      const resp = await apiRequest<InviteCreated>(
        `/api/v1/elections/${id}/invites`,
        { method: "POST", body: JSON.stringify({ email: inviteEmail.trim() }) },
        token
      );
      setInviteCode(resp.invite_code || null);
      setInviteEmail("");
      await reload();
    } catch (e: any) {
      setErr(e?.message || "create invite failed");
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
        {loading ? <div style={styles.muted}>Loading…</div> : null}

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
                    close (triggers tally)
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
                        placeholder="email to invite"
                      />
                      <button style={styles.btnPrimary} onClick={createInvite} disabled={loading}>
                        Create invite
                      </button>
                    </div>

                    {inviteCode ? (
                      <div style={{ marginTop: 10, ...styles.card, borderColor: "#bbf7d0", background: "#f0fdf4" }}>
                        <div style={{ fontWeight: 700 }}>Invite code (один раз показываем):</div>
                        <div style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>
                          {inviteCode}
                        </div>
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
                            created_at: {it.created_at}
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

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Raw election JSON</h3>
        {item ? <JsonBlock value={item} /> : <div style={styles.muted}>—</div>}
      </div>
    </div>
  );
}

function VoteView({ token, me, id, go }: { token: string | null; me: Me | null; id: string; go: (to: string) => void }) {
  const [meta, setMeta] = useState<BallotMeta | null>(null);
  const [my, setMy] = useState<MyBallotResp | null>(null);

  const [approvalSet, setApprovalSet] = useState<string[]>([]);
  const [ranking, setRanking] = useState<string[]>([]);
  const [scores, setScores] = useState<Record<string, number>>({});

  const [idemKey, setIdemKey] = useState<string>(() => newIdemKey());

  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  const [submitResp, setSubmitResp] = useState<any>(null);

  const reload = async () => {
    setLoading(true);
    setErr(null);
    setSubmitResp(null);
    try {
      const m = await apiRequest<BallotMeta>(`/api/v1/elections/${id}/ballot`, { method: "GET" }, token);
      setMeta(m);

      const mb = await apiRequest<MyBallotResp>(
        `/api/v1/elections/${id}/ballots/me`,
        { method: "GET" },
        token
      );
      setMy(mb);

      if (m.ballot_format === "score" && m.score_min != null) {
        const next: Record<string, number> = {};
        for (const c of m.candidates) {
          next[c.id] = m.score_min;
        }
        setScores(next);
      }
    } catch (e: any) {
      setErr(e?.message || "load ballot meta failed");
      setMeta(null);
      setMy(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, [id]);

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

  const submit = async () => {
    if (!meta) return;
    setLoading(true);
    setErr(null);
    setSubmitResp(null);

    try {
      const body: AnyObj = {};
      if (meta.ballot_format === "approval") body.approval_set = approvalSet;
      if (meta.ballot_format === "ranking") body.ranking = ranking;
      if (meta.ballot_format === "score") body.scores = scores;

      const resp = await apiRequest<any>(
        `/api/v1/elections/${id}/ballots/submit`,
        {
          method: "POST",
          headers: { "Idempotency-Key": idemKey },
          body: JSON.stringify(body),
        },
        token
      );

      setSubmitResp(resp);
      setIdemKey(newIdemKey());
      await reload();
    } catch (e: any) {
      setErr(e?.message || "submit failed");
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

        {loading ? <div style={styles.muted}>Loading…</div> : null}

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
              {meta.ballot_format === "approval" && meta.approval_max_choices != null ? (
                <Badge text={`max choices: ${meta.approval_max_choices}`} />
              ) : null}
              {meta.ballot_format === "ranking" && meta.ranking_top_k != null ? (
                <Badge text={`topK: ${meta.ranking_top_k}`} />
              ) : null}
              {meta.ballot_format === "score" ? (
                <Badge
                  text={`score: ${meta.score_min ?? "?"}..${meta.score_max ?? "?"} step ${
                    meta.score_step ?? "?"
                  } allow_skip=${String(meta.score_allow_skip)}`}
                />
              ) : null}
            </div>

            <hr style={styles.hr} />

            {meta.ballot_format === "approval" ? (
              <>
                <h3 style={{ marginTop: 0 }}>Approval</h3>
                <div style={styles.muted}>
                  Выбери кандидатов. Лимит: {meta.approval_max_choices ?? "?"}
                </div>
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
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() => toggleApproval(c.id)}
                        />
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
                <div style={styles.muted}>
                  Кликаешь кандидата — добавляется в конец ранжирования. Повторный клик — удаляет.
                  TopK: {meta.ranking_top_k ?? "?"}
                </div>

                <div style={{ marginTop: 10, ...styles.card, background: "#f9fafb" }}>
                  <div style={{ fontWeight: 700 }}>Текущее ранжирование:</div>
                  {ranking.length === 0 ? (
                    <div style={styles.muted}>—</div>
                  ) : (
                    <ol style={{ margin: "6px 0 0 18px" }}>
                      {ranking.map((cid) => {
                        const c = meta.candidates.find((x) => x.id === cid);
                        return (
                          <li key={cid}>
                            {c ? `${c.name}` : cid}{" "}
                            <span style={{ ...styles.muted, fontSize: 12 }}>({cid})</span>
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
                <div style={styles.muted}>
                  Выставь оценки. Allow-skip: {String(meta.score_allow_skip)}.
                </div>

                <div style={{ marginTop: 10, display: "grid", gap: 10 }}>
                  {meta.candidates.map((c) => {
                    const v = scores[c.id];
                    const min = meta.score_min ?? 0;
                    const max = meta.score_max ?? 10;
                    const step = meta.score_step ?? 1;

                    const isMissing = !meta.score_allow_skip && (v === undefined || v === null);
                    return (
                      <div
                        key={c.id}
                        style={{
                          ...styles.card,
                          padding: 12,
                          borderColor: isMissing ? "#fecaca" : "#e5e7eb",
                          background: isMissing ? "#fff1f2" : "white",
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
                              placeholder={meta.score_allow_skip ? "skip allowed" : "required"}
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
                <b>Idempotency-Key:</b>
                <span style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{idemKey}</span>
                <button style={styles.btn} onClick={() => setIdemKey(newIdemKey())} disabled={loading}>
                  regen
                </button>
              </div>

              <button style={styles.btnPrimary} onClick={submit} disabled={loading}>
                {loading ? "Submitting…" : "Submit ballot"}
              </button>
            </div>

            {submitResp ? (
              <>
                <hr style={styles.hr} />
                <h3 style={{ marginTop: 0 }}>Submit response</h3>
                <JsonBlock value={submitResp} />
              </>
            ) : null}
          </>
        ) : null}
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Debug meta/my</h3>
        <div style={{ display: "grid", gap: 10 }}>
          <div>
            <div style={styles.muted}>Ballot meta</div>
            {meta ? <JsonBlock value={meta} /> : <div style={styles.muted}>—</div>}
          </div>
          <div>
            <div style={styles.muted}>My ballot</div>
            {my ? <JsonBlock value={my} /> : <div style={styles.muted}>—</div>}
          </div>
        </div>
      </div>
    </div>
  );
}

function ResultsView({ token, id, go }: { token: string | null; me: Me | null; id: string; go: (to: string) => void }) {
  const [res, setRes] = useState<ResultResp | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const reload = async () => {
    setLoading(true);
    setErr(null);
    try {
      const r = await apiRequest<ResultResp>(`/api/v1/elections/${id}/results`, { method: "GET" }, token);
      setRes(r);
    } catch (e: any) {
      setErr(e?.message || "get results failed");
      setRes(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    reload();
  }, [id]);

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
        {loading ? <div style={styles.muted}>Loading…</div> : null}

        {res ? (
          <>
            <div style={{ marginTop: 10, display: "flex", gap: 8, flexWrap: "wrap" }}>
              <Badge text={`method: ${res.method}`} />
              <Badge text={`version: ${String(res.version)}`} />
              {res.published_at ? <Badge text={`published_at: ${res.published_at}`} /> : <Badge text="published_at: null" />}
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

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Raw results JSON</h3>
        {res ? <JsonBlock value={res} /> : <div style={styles.muted}>—</div>}
      </div>
    </div>
  );
}

function AdminCreateElection({ token, me, go }: { token: string | null; me: Me | null; go: (to: string) => void }) {
  const [title, setTitle] = useState("Demo election");
  const [description, setDescription] = useState("Created from demo UI");

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
  const [rawResp, setRawResp] = useState<any>(null);

  useEffect(() => {
    if (ballotFormat === "approval") {
      setTallyRule("approval");
    } else if (ballotFormat === "ranking") {
      if (tallyRule === "approval") setTallyRule("plurality");
    } else if (ballotFormat === "score") {
      if (tallyRule !== "plurality" && tallyRule !== "borda" && tallyRule !== "approval") {
        setTallyRule("plurality");
      }
    }
  }, [ballotFormat]);

  const submit = async () => {
    if (me?.role !== "admin") {
      setErr("only admin can create elections");
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

      const candidates = names.map((name) => ({ name }));

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

      const resp = await apiRequest<any>(
        "/api/v1/elections",
        { method: "POST", body: JSON.stringify(body) },
        token
      );
      setRawResp(resp);

      const eid = resp?.id;
      if (typeof eid === "string" && eid.trim()) {
        setCreatedID(eid.trim());
      } else {
        throw new Error("create ok, но не нашёл id в ответе");
      }
    } catch (e: any) {
      setErr(e?.message || "create election failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: "grid", gap: 12 }}>
      <div style={styles.card}>
        <div style={{ display: "flex", justifyContent: "space-between", gap: 10, alignItems: "baseline" }}>
          <h2 style={{ margin: 0 }}>Create election (admin)</h2>
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
            <div style={{ marginTop: 6, ...styles.muted, fontSize: 12 }}>
              Для MVP tally на бэке: ranking plurality/borda, approval -> approval.
            </div>
          </div>

          <div>
            <label>Committee size</label>
            <input
              style={styles.input}
              type="number"
              min={1}
              value={committeeSize}
              onChange={(e) => setCommitteeSize(Number(e.target.value))}
            />
          </div>
          <div>
            <label>Quota type (если committee_size &gt; 1)</label>
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
              <input
                type="checkbox"
                checked={showAggregates}
                onChange={(e) => setShowAggregates(e.target.checked)}
              />
              show_aggregates
            </label>
          </div>
        </div>

        <hr style={styles.hr} />

        {ballotFormat === "approval" ? (
          <div style={styles.grid2}>
            <div>
              <label>approval_max_choices</label>
              <input
                style={styles.input}
                type="number"
                min={1}
                value={approvalMax}
                onChange={(e) => setApprovalMax(Number(e.target.value))}
              />
            </div>
            <div />
          </div>
        ) : null}

        {ballotFormat === "ranking" ? (
          <div style={styles.grid2}>
            <div>
              <label>ranking_top_k</label>
              <input
                style={styles.input}
                type="number"
                min={1}
                value={rankingTopK}
                onChange={(e) => setRankingTopK(Number(e.target.value))}
              />
            </div>
            <div />
          </div>
        ) : null}

        {ballotFormat === "score" ? (
          <div style={styles.grid2}>
            <div>
              <label>score_min</label>
              <input
                style={styles.input}
                type="number"
                value={scoreMin}
                onChange={(e) => setScoreMin(Number(e.target.value))}
              />
            </div>
            <div>
              <label>score_max</label>
              <input
                style={styles.input}
                type="number"
                value={scoreMax}
                onChange={(e) => setScoreMax(Number(e.target.value))}
              />
            </div>
            <div>
              <label>score_step</label>
              <input
                style={styles.input}
                type="number"
                min={1}
                value={scoreStep}
                onChange={(e) => setScoreStep(Number(e.target.value))}
              />
            </div>
            <div style={{ display: "flex", alignItems: "center" }}>
              <label style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <input
                  type="checkbox"
                  checked={scoreAllowSkip}
                  onChange={(e) => setScoreAllowSkip(e.target.checked)}
                />
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
            {loading ? "Creating…" : "Create"}
          </button>
          {createdID ? (
            <button style={styles.btn} onClick={() => go(`#/elections/${createdID}`)}>
              Open created election
            </button>
          ) : null}
        </div>
      </div>

      <div style={styles.card}>
        <h3 style={{ marginTop: 0 }}>Raw create response</h3>
        {rawResp ? <JsonBlock value={rawResp} /> : <div style={styles.muted}>—</div>}
      </div>
    </div>
  );
}
