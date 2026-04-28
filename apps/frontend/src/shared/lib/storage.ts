import type { AuthTokens } from "../api/types";

const STORAGE_AUTH_KEY = "sv_auth_tokens";
const LEGACY_STORAGE_TOKEN_KEY = "sv_token";

function isAuthTokens(value: unknown): value is AuthTokens {
  if (!value || typeof value !== "object") return false;
  const v = value as Partial<AuthTokens>;

  return (
    typeof v.access_token === "string" &&
    v.access_token.trim() !== "" &&
    typeof v.expires_at === "string" &&
    typeof v.refresh_token === "string" &&
    typeof v.refresh_expires_at === "string"
  );
}

export function readStoredAuthTokens(): AuthTokens | null {
  const raw = localStorage.getItem(STORAGE_AUTH_KEY);

  if (raw) {
    try {
      const parsed = JSON.parse(raw) as unknown;
      if (isAuthTokens(parsed)) {
        return {
          access_token: parsed.access_token.trim(),
          expires_at: parsed.expires_at,
          refresh_token: parsed.refresh_token.trim(),
          refresh_expires_at: parsed.refresh_expires_at,
        };
      }
    } catch {
      localStorage.removeItem(STORAGE_AUTH_KEY);
    }
  }

  const legacyToken = localStorage.getItem(LEGACY_STORAGE_TOKEN_KEY);
  if (legacyToken && legacyToken.trim()) {
    return {
      access_token: legacyToken.trim(),
      expires_at: "",
      refresh_token: "",
      refresh_expires_at: "",
    };
  }

  return null;
}

export function writeStoredAuthTokens(tokens: AuthTokens) {
  localStorage.setItem(
    STORAGE_AUTH_KEY,
    JSON.stringify({
      access_token: tokens.access_token,
      expires_at: tokens.expires_at,
      refresh_token: tokens.refresh_token,
      refresh_expires_at: tokens.refresh_expires_at,
    })
  );
  localStorage.removeItem(LEGACY_STORAGE_TOKEN_KEY);
}

export function clearStoredAuthTokens() {
  localStorage.removeItem(STORAGE_AUTH_KEY);
  localStorage.removeItem(LEGACY_STORAGE_TOKEN_KEY);
}

export function readStoredToken(): string | null {
  return readStoredAuthTokens()?.access_token ?? null;
}

export function writeStoredToken(token: string) {
  writeStoredAuthTokens({
    access_token: token,
    expires_at: "",
    refresh_token: "",
    refresh_expires_at: "",
  });
}

export function clearStoredToken() {
  clearStoredAuthTokens();
}