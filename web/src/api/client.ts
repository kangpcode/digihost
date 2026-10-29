// DigiPanel Centralized API Client
// Default-deny authentication header enforcement & type-safe request dispatcher
// Zero emojis, strictly production error handling

export const AUTH_TOKEN = 'digipanel-secure-session-token-v1';

export async function apiFetch<T = any>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {});
  
  if (!headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${AUTH_TOKEN}`);
  }
  
  if (!headers.has('Content-Type') && !(options.body instanceof FormData)) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(endpoint, {
    ...options,
    headers,
  });

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

  return response.json();
}
