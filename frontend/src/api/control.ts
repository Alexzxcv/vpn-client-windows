import type {
  Bootstrap,
  ConnectResult,
  ConnMode,
  Location,
  Me,
  Proxy,
  Settings,
  Status,
  UpdateResult,
  Usage,
} from './types';

export class ApiError extends Error {
  readonly status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

interface ApiErrorBody {
  error?: string;
  message?: string;
}

/**
 * Клиент локального control-API Go-ядра.
 * Базовый URL относительный (`/api/...`), т.к. UI отдаётся тем же ядром (same-origin).
 * Хранит session_token в памяти и шлёт его как `Authorization: Bearer <token>`.
 */
export class ControlApi {
  private sessionToken: string | null = null;

  setSessionToken(token: string | null): void {
    this.sessionToken = token;
  }

  hasSessionToken(): boolean {
    return this.sessionToken !== null;
  }

  private async request<T>(
    path: string,
    options: { method?: string; body?: unknown; auth?: boolean } = {},
  ): Promise<T> {
    const { method = 'GET', body, auth = true } = options;

    const headers: Record<string, string> = {};
    if (body !== undefined) {
      headers['Content-Type'] = 'application/json';
    }
    if (auth) {
      if (!this.sessionToken) {
        throw new ApiError('No session token', 401);
      }
      headers['Authorization'] = `Bearer ${this.sessionToken}`;
    }

    let res: Response;
    try {
      res = await fetch(path, {
        method,
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
    } catch (e) {
      throw new ApiError(
        e instanceof Error ? e.message : 'Network error',
        0,
      );
    }

    if (!res.ok) {
      let msg = `Request failed (${res.status})`;
      try {
        const data = (await res.json()) as ApiErrorBody;
        if (data.error) msg = data.error;
        else if (data.message) msg = data.message;
      } catch {
        // тело не JSON — оставляем дефолтное сообщение
      }
      throw new ApiError(msg, res.status);
    }

    if (res.status === 204) {
      return undefined as T;
    }

    const text = await res.text();
    if (!text) {
      return undefined as T;
    }
    return JSON.parse(text) as T;
  }

  // --- Эндпоинты ---

  bootstrap(): Promise<Bootstrap> {
    return this.request<Bootstrap>('/api/bootstrap', { auth: false });
  }

  /** Open an http(s) URL in the system browser (e.g. the web dashboard). */
  openExternal(url: string): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>('/api/open-external', {
      method: 'POST',
      body: { url },
    });
  }

  status(): Promise<Status> {
    return this.request<Status>('/api/status');
  }

  login(login: string, password: string): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>('/api/auth/login', {
      method: 'POST',
      body: { login, password },
    });
  }

  logout(): Promise<void> {
    return this.request<void>('/api/auth/logout', { method: 'POST' });
  }

  me(): Promise<Me> {
    return this.request<Me>('/api/me');
  }

  locations(): Promise<Location[]> {
    return this.request<Location[]>('/api/locations');
  }

  usage(hours = 24): Promise<Usage> {
    return this.request<Usage>(`/api/usage?hours=${hours}`);
  }

  connect(serverId?: string, mode?: ConnMode): Promise<ConnectResult> {
    const body: { server_id?: string; mode?: ConnMode } = {};
    if (serverId) body.server_id = serverId;
    if (mode) body.mode = mode;
    return this.request<ConnectResult>('/api/connect', {
      method: 'POST',
      body,
    });
  }

  disconnect(): Promise<ConnectResult> {
    return this.request<ConnectResult>('/api/disconnect', { method: 'POST' });
  }

  proxy(): Promise<Proxy> {
    return this.request<Proxy>('/api/proxy');
  }

  getSettings(): Promise<Settings> {
    return this.request<Settings>('/api/settings');
  }

  saveSettings(s: Settings): Promise<Settings> {
    return this.request<Settings>('/api/settings', {
      method: 'PUT',
      body: s,
    });
  }

  checkUpdate(): Promise<UpdateResult> {
    return this.request<UpdateResult>('/api/update/check');
  }

  applyUpdate(): Promise<{ ok: boolean }> {
    return this.request<{ ok: boolean }>('/api/update/apply', {
      method: 'POST',
    });
  }

  // --- Native window controls (custom frameless title bar) ---

  windowMinimize(): Promise<void> {
    return this.request<void>('/api/window/minimize', { method: 'POST' });
  }

  windowMaximize(): Promise<void> {
    return this.request<void>('/api/window/maximize', { method: 'POST' });
  }

  windowClose(): Promise<void> {
    return this.request<void>('/api/window/close', { method: 'POST' });
  }
}

export const api = new ControlApi();
