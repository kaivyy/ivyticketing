// Session token accessor.
//
// The scanner authenticates once (Login.svelte, task 13) and stores the bearer
// token in Local_Store. `lib/api.ts` reads the current token through the
// accessor injected here so the transport layer stays decoupled from storage.

const TOKEN_KEY = 'ivy.scanner.token';

/** Returns the stored session bearer token, or null when signed out. */
export function getSessionToken(): string | null {
  try {
    return localStorage.getItem(TOKEN_KEY);
  } catch {
    return null;
  }
}

/** Persists the session bearer token in Local_Store. */
export function setSessionToken(token: string): void {
  try {
    localStorage.setItem(TOKEN_KEY, token);
  } catch {
    /* storage unavailable (e.g. private mode); ignore */
  }
}

/** Clears the stored session token (logout). */
export function clearSessionToken(): void {
  try {
    localStorage.removeItem(TOKEN_KEY);
  } catch {
    /* ignore */
  }
}
