import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../shared/api/client";
import { clearStoredToken, readStoredToken, writeStoredToken } from "../shared/lib/storage";
import type { Me } from "../shared/api/types";

type AuthCtx = {
  token: string | null;
  me: Me | null;
  authed: boolean;
  bootLoading: boolean;
  bootError: string | null;
  setToken: (t: string | null) => void;
  logout: () => Promise<void>;
};

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [token, setTokenState] = useState<string | null>(() => readStoredToken());
  const [me, setMe] = useState<Me | null>(null);
  const [bootLoading, setBootLoading] = useState(false);
  const [bootError, setBootError] = useState<string | null>(null);

  const authed = Boolean(token && me?.email);
  const bootAbortRef = useRef<AbortController | null>(null);

  const setToken = useCallback((t: string | null) => {
    if (!t) {
      clearStoredToken();
      setTokenState(null);
      setMe(null);
      return;
    }
    writeStoredToken(t);
    setTokenState(t);
  }, []);

  const clearAuth = useCallback(() => {
    clearStoredToken();
    setTokenState(null);
    setMe(null);
  }, []);

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
        const user = await api.auth.me(token, ac.signal);
        setMe(user);
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

  const logout = useCallback(async () => {
    try {
      if (token) await api.auth.logout(token);
    } finally {
      clearAuth();
    }
  }, [token, clearAuth]);

  const value = useMemo<AuthCtx>(
    () => ({ token, me, authed, bootLoading, bootError, setToken, logout }),
    [token, me, authed, bootLoading, bootError, setToken, logout]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth() {
  const v = useContext(Ctx);
  if (!v) throw new Error("AuthProvider is missing");
  return v;
}