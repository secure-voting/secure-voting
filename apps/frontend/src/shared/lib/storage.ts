const STORAGE_TOKEN_KEY = "sv_token";

export function readStoredToken(): string | null {
  const t = localStorage.getItem(STORAGE_TOKEN_KEY);
  return t && t.trim() ? t.trim() : null;
}

export function writeStoredToken(token: string) {
  localStorage.setItem(STORAGE_TOKEN_KEY, token);
}

export function clearStoredToken() {
  localStorage.removeItem(STORAGE_TOKEN_KEY);
}