// DigiPanel Centralized API Client
// Session tokens are issued by the core login endpoint and stored per browser
// session. No credential or token is compiled into the bundle, and an expired
// session is surfaced to the shell so it can return to the login screen.

export const SESSION_STORAGE_KEY = 'digipanel.session';

export interface SessionToken {
  token: string;
  username: string;
  role: string;
}

export class UnauthorizedError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'UnauthorizedError';
  }
}

export const readSession = (): SessionToken | null => {
  try {
    const raw = window.sessionStorage.getItem(SESSION_STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as SessionToken;
    if (!parsed || typeof parsed.token !== 'string' || parsed.token === '') return null;
    return parsed;
  } catch {
    return null;
  }
};

export const storeSession = (session: SessionToken): void => {
  window.sessionStorage.setItem(SESSION_STORAGE_KEY, JSON.stringify(session));
};

export const clearSession = (): void => {
  window.sessionStorage.removeItem(SESSION_STORAGE_KEY);
};

// onUnauthorized is invoked once per failed request whose cause is a missing or
// expired session, so the shell can drop back to the login screen.
let unauthorizedHandler: (() => void) | null = null;

export const setUnauthorizedHandler = (handler: (() => void) | null): void => {
  unauthorizedHandler = handler;
};

export async function apiFetch<T = any>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {});
  const session = readSession();

  if (session) {
    headers.set('Authorization', `Bearer ${session.token}`);
  }

  if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(endpoint, {
    ...options,
    headers,
  });

  if (response.status === 401) {
    clearSession();
    if (unauthorizedHandler) unauthorizedHandler();
    throw new UnauthorizedError('Your session has ended. Sign in again to continue.');
  }

  if (!response.ok) {
    const errorText = await response.text();
    let errorMsg = `HTTP ${response.status}: ${response.statusText}`;
    try {
      const parsed = JSON.parse(errorText);
      if (parsed.error) errorMsg = parsed.error;
    } catch {
      if (errorText) errorMsg = errorText;
    }
    throw new Error(errorMsg);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json();
}

// login exchanges credentials for a session token and persists it.
export async function login(username: string, password: string, totpCode?: string): Promise<SessionToken> {
  const payload: Record<string, string> = { username, password };
  if (totpCode) payload.totpCode = totpCode;

  const response = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    let message = 'Sign in failed.';
    if (response.status === 401) message = 'Incorrect username or password.';
    if (response.status === 429) message = 'Too many failed attempts. Try again later.';
    try {
      const parsed = await response.json();
      if (parsed.error) message = parsed.error;
    } catch {
      // Keep the status based message.
    }
    throw new Error(message);
  }

  const data = await response.json() as { token: string; username: string; role: string };
  const session: SessionToken = { token: data.token, username: data.username, role: data.role };
  storeSession(session);
  return session;
}

// logout revokes the session on the server, then clears it locally.
export async function logout(): Promise<void> {
  try {
    if (readSession()) {
      await apiFetch('/api/v1/auth/logout', { method: 'POST', body: JSON.stringify({}) });
    }
  } catch {
    // A revoked or expired session is already signed out.
  } finally {
    clearSession();
  }
}
