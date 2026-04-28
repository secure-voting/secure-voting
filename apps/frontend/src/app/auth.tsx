import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../shared/api/client";
import {
  clearStoredAuthTokens,
  readStoredAuthTokens,
  writeStoredAuthTokens,
} from "../shared/lib/storage";
import type { AuthTokens, Me } from "../shared/api/types";

type AuthCtx = {
  token: string | null;
  authTokens: AuthTokens | null;
  me: Me | null;
  authed: boolean;
  bootLoading: boolean;
  bootError: string | null;
  setToken: (t: AuthTokens | string | null) => void;
  logout: () => Promise<void>;
};

const Ctx = createContext<AuthCtx | null>(null);

function normalizeAuthTokens(value: AuthTokens | string | null): AuthTokens | null {
  if (!value) return null;

  if (typeof value === "string") {
    const accessToken = value.trim();
    if (!accessToken) return null;

    return {
      access_token: accessToken,
      expires_at: "",
      refresh_token: "",
      refresh_expires_at: "",
    };
  }

  const accessToken = value.access_token?.trim();
  if (!accessToken) return null;

  return {
    access_token: accessToken,
    expires_at: value.expires_at || "",
    refresh_token: value.refresh_token?.trim() || "",
    refresh_expires_at: value.refresh_expires_at || "",
  };
}

function millisecondsUntilRefresh(expiresAt: string): number | null {
  if (!expiresAt) return null;

  const expiresMs = Date.parse(expiresAt);
  if (!Number.isFinite(expiresMs)) return null;

  const refreshMs = expiresMs - Date.now() - 60_000;
  return Math.max(0, refreshMs);
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [authTokens, setAuthTokensState] = useState<AuthTokens | null>(() => readStoredAuthTokens());
  const [me, setMe] = useState<Me | null>(null);
  const [bootLoading, setBootLoading] = useState(false);
  const [bootError, setBootError] = useState<string | null>(null);

  const token = authTokens?.access_token ?? null;
  const authed = Boolean(token && me?.email);

  const bootAbortRef = useRef<AbortController | null>(null);
  const refreshTimerRef = useRef<number | null>(null);
  const refreshInFlightRef = useRef<Promise<AuthTokens> | null>(null);

  const clearAuth = useCallback(() => {
    clearStoredAuthTokens();
    setAuthTokensState(null);
    setMe(null);
  }, []);

  const setToken = useCallback((next: AuthTokens | string | null) => {
    const normalized = normalizeAuthTokens(next);

    if (!normalized) {
      clearStoredAuthTokens();
      setAuthTokensState(null);
      setMe(null);
      return;
    }

    writeStoredAuthTokens(normalized);
    setAuthTokensState(normalized);
  }, []);

  const refreshTokens = useCallback(async (refreshToken: string) => {
    if (!refreshToken.trim()) {
      throw new Error("Refresh token отсутствует");
    }

    if (!refreshInFlightRef.current) {
      refreshInFlightRef.current = api.auth
        .refresh(refreshToken)
        .finally(() => {
          refreshInFlightRef.current = null;
        });
    }

    return await refreshInFlightRef.current;
  }, []);

  useEffect(() => {
    bootAbortRef.current?.abort();
    bootAbortRef.current = null;

    if (!authTokens?.access_token) {
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
        const user = await api.auth.me(authTokens.access_token, ac.signal);
        setMe(user);
      } catch (e: any) {
        if (e?.name === "AbortError") return;

        const canRefresh = Boolean(authTokens.refresh_token);
        const shouldRefresh = e?.status === 401 && canRefresh;

        if (!shouldRefresh) {
          setBootError(e?.message || "Не удалось проверить сессию");
          clearAuth();
          return;
        }

        try {
          const refreshed = await refreshTokens(authTokens.refresh_token);
          if (ac.signal.aborted) return;

          writeStoredAuthTokens(refreshed);
          setAuthTokensState(refreshed);

          const user = await api.auth.me(refreshed.access_token, ac.signal);
          if (ac.signal.aborted) return;

          setMe(user);
        } catch (refreshErr: any) {
          if (refreshErr?.name === "AbortError") return;
          setBootError(refreshErr?.message || "Сессия истекла, выполните вход заново");
          clearAuth();
        }
      } finally {
        setBootLoading(false);
      }
    };

    run();

    return () => {
      ac.abort();
      bootAbortRef.current = null;
    };
  }, [authTokens, clearAuth, refreshTokens]);

  useEffect(() => {
    if (refreshTimerRef.current) {
      window.clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }

    if (!authTokens?.refresh_token) return;

    const delay = millisecondsUntilRefresh(authTokens.expires_at);
    if (delay == null) return;

    refreshTimerRef.current = window.setTimeout(async () => {
      try {
        const refreshed = await refreshTokens(authTokens.refresh_token);
        writeStoredAuthTokens(refreshed);
        setAuthTokensState(refreshed);
      } catch {
        clearAuth();
      }
    }, delay);

    return () => {
      if (refreshTimerRef.current) {
        window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };
  }, [authTokens, clearAuth, refreshTokens]);

  const logout = useCallback(async () => {
    try {
      if (token) await api.auth.logout(token);
    } finally {
      clearAuth();
    }
  }, [token, clearAuth]);

  const value = useMemo<AuthCtx>(
    () => ({
      token,
      authTokens,
      me,
      authed,
      bootLoading,
      bootError,
      setToken,
      logout,
    }),
    [token, authTokens, me, authed, bootLoading, bootError, setToken, logout]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth() {
  const v = useContext(Ctx);
  if (!v) throw new Error("AuthProvider is missing");
  return v;
}