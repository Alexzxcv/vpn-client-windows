import { makeAutoObservable, runInAction } from 'mobx';
import type { ControlApi } from '@/api/control';
import { ApiError } from '@/api/control';
import type { MultiProxyEntry } from '@/api/types';
import { i18n } from './I18nStore';

const POLL_INTERVAL_MS = 2000;

/**
 * Manages the local multi-proxy entries: several SOCKS5 listeners on different
 * ports, each pointed at its own server. Polls the core every 2s for the live
 * state of each entry. CRUD/start/stop surface user-facing failures via
 * `actionError`; background polling failures stay silent.
 */
export class MultiProxyStore {
  private readonly api: ControlApi;
  private pollTimer: ReturnType<typeof setInterval> | null = null;

  entries: MultiProxyEntry[] = [];
  busy = false;
  actionError: string | null = null;

  constructor(api: ControlApi) {
    this.api = api;
    makeAutoObservable(this, {}, { autoBind: true });
  }

  startPolling(): void {
    if (this.pollTimer !== null) return;
    void this.refresh();
    this.pollTimer = setInterval(() => {
      void this.refresh();
    }, POLL_INTERVAL_MS);
  }

  stopPolling(): void {
    if (this.pollTimer !== null) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  /** Reload the entry list. Silent on transient failures (background polling). */
  async refresh(): Promise<void> {
    if (!this.api.hasSessionToken()) return;
    try {
      const entries = await this.api.listMultiProxy();
      runInAction(() => {
        this.entries = entries;
      });
    } catch {
      // поллинг — тихо игнорируем единичные сбои
    }
  }

  clearError(): void {
    this.actionError = null;
  }

  async add(port: number, serverId: string, main: boolean): Promise<boolean> {
    this.busy = true;
    this.actionError = null;
    try {
      await this.api.addMultiProxy(port, serverId, main);
      await this.refresh();
      return true;
    } catch (e) {
      runInAction(() => {
        this.actionError =
          e instanceof Error ? e.message : i18n.t('multiproxy.addFailed');
      });
      return false;
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }

  async update(
    id: string,
    port: number,
    serverId: string,
    main: boolean,
  ): Promise<boolean> {
    this.busy = true;
    this.actionError = null;
    try {
      await this.api.updateMultiProxy(id, port, serverId, main);
      await this.refresh();
      return true;
    } catch (e) {
      runInAction(() => {
        this.actionError =
          e instanceof Error ? e.message : i18n.t('multiproxy.updateFailed');
      });
      return false;
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }

  async remove(id: string): Promise<void> {
    this.busy = true;
    this.actionError = null;
    try {
      await this.api.removeMultiProxy(id);
      await this.refresh();
    } catch (e) {
      runInAction(() => {
        this.actionError =
          e instanceof Error ? e.message : i18n.t('multiproxy.removeFailed');
      });
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }

  async start(id: string): Promise<void> {
    this.busy = true;
    this.actionError = null;
    try {
      await this.api.startMultiProxy(id);
      await this.refresh();
    } catch (e) {
      runInAction(() => {
        // Friendly message for the per-tier device limit (backend nodes only).
        if (e instanceof ApiError && e.code === 'device_limit') {
          this.actionError = i18n.t('connect.deviceLimit');
        } else {
          this.actionError =
            e instanceof Error ? e.message : i18n.t('multiproxy.startFailed');
        }
      });
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }

  async stop(id: string): Promise<void> {
    this.busy = true;
    this.actionError = null;
    try {
      await this.api.stopMultiProxy(id);
      await this.refresh();
    } catch (e) {
      runInAction(() => {
        this.actionError =
          e instanceof Error ? e.message : i18n.t('multiproxy.stopFailed');
      });
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }
}
