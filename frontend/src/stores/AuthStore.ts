import { makeAutoObservable, runInAction } from 'mobx';
import { ApiError, type ControlApi } from '@/api/control';
import type { Me } from '@/api/types';

export class AuthStore {
  private readonly api: ControlApi;

  bootstrapped = false;
  bootstrapError: string | null = null;
  version = '';
  /** Whether the core runs as administrator (TUN mode availability). */
  elevated = false;

  authenticated = false;
  me: Me | null = null;

  loggingIn = false;
  loginError: string | null = null;

  constructor(api: ControlApi) {
    this.api = api;
    makeAutoObservable(this, {}, { autoBind: true });
  }

  /** Получает session_token у ядра и сохраняет его в API-клиенте. */
  async bootstrap(): Promise<void> {
    try {
      const data = await this.api.bootstrap();
      this.api.setSessionToken(data.session_token);
      // Recognize an already-authenticated core (persisted tokens) BEFORE
      // flipping `bootstrapped` — otherwise the router mounts with
      // authenticated=false and bounces the user to /login (race).
      let authed = false;
      try {
        authed = (await this.api.status()).authenticated;
      } catch {
        // treat as not authenticated
      }
      runInAction(() => {
        this.version = data.version;
        this.elevated = data.elevated;
        this.authenticated = authed;
        this.bootstrapped = true;
        this.bootstrapError = null;
      });
      if (authed) void this.loadMe();
    } catch (e) {
      runInAction(() => {
        this.bootstrapError =
          e instanceof Error ? e.message : 'Bootstrap failed';
        this.bootstrapped = false;
      });
    }
  }

  setAuthenticated(value: boolean): void {
    this.authenticated = value;
    if (!value) {
      this.me = null;
    }
  }

  async login(email: string, password: string): Promise<boolean> {
    this.loggingIn = true;
    this.loginError = null;
    try {
      await this.api.login(email, password);
      runInAction(() => {
        this.authenticated = true;
        this.loggingIn = false;
      });
      void this.loadMe();
      return true;
    } catch (e) {
      runInAction(() => {
        this.loginError =
          e instanceof ApiError
            ? e.message
            : e instanceof Error
              ? e.message
              : 'Login failed';
        this.loggingIn = false;
      });
      return false;
    }
  }

  async logout(): Promise<void> {
    try {
      await this.api.logout();
    } catch {
      // даже при ошибке считаем сессию завершённой локально
    }
    runInAction(() => {
      this.authenticated = false;
      this.me = null;
    });
  }

  async loadMe(): Promise<void> {
    try {
      const me = await this.api.me();
      runInAction(() => {
        this.me = me;
      });
    } catch {
      // некритично
    }
  }
}
