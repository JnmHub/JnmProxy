export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

const API_PREFIX = '/api/v1';
const ADMIN_TOKEN_KEY = 'jnmproxy.admin_token';

type RequestOptions = Omit<RequestInit, 'body'> & {
  body?: unknown;
};

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  const token = getAdminToken();
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json');
  }

  const response = await fetch(`${API_PREFIX}${path}`, {
    ...options,
    headers,
    body: options.body === undefined ? undefined : JSON.stringify(options.body),
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  const data = text ? JSON.parse(text) : undefined;
  if (!response.ok) {
    const message = typeof data?.error === 'string' ? data.error : `请求失败：${response.status}`;
    throw new ApiError(message, response.status);
  }
  return data as T;
}

export function getAdminToken() {
  if (typeof window === 'undefined') return '';
  return window.localStorage.getItem(ADMIN_TOKEN_KEY) ?? '';
}

export function saveAdminToken(token: string) {
  if (typeof window === 'undefined') return;
  const value = token.trim();
  if (value) {
    window.localStorage.setItem(ADMIN_TOKEN_KEY, value);
  } else {
    window.localStorage.removeItem(ADMIN_TOKEN_KEY);
  }
}
