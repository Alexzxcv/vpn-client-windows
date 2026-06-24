import { makeAutoObservable, runInAction } from 'mobx';
import type { ControlApi } from '@/api/control';
import type {
  ConnMode,
  ConnState,
  Location,
  Proxy,
  Status,
} from '@/api/types';
import type { AuthStore } from './AuthStore';

const POLL_INTERVAL_MS = 2000;

export class ConnectionStore {
  private readonly api: ControlApi;
  private readonly auth: AuthStore;
  private pollTimer: ReturnType<typeof setInterval> | null = null;

  state: ConnState = 'disconnected';
  connected = false;
  lastError: string | null = null;
  since: string | null = null;

  /** Actual active mode reported by the core. */
  mode: ConnMode = 'proxy';
  /** Mode the user picked for the next connect. */
  selectedMode: ConnMode = 'proxy';

  locations: Location[] = [];
  selectedServerId: string | null = null;
  proxy: Proxy | null = null;

  busy = false;
  actionError: string | null = null;

  constructor(api: ControlApi, auth: AuthStore) {
    this.api = api;
    this.auth = auth;
    makeAutoObservable(this, {}, { autoBind: true });
  }

  startPolling(): void {
    if (this.pollTimer !== null) return;
    void this.refreshStatus();
    this.pollTimer = setInterval(() => {
      void this.refreshStatus();
    }, POLL_INTERVAL_MS);
  }

  stopPolling(): void {
    if (this.pollTimer !== null) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
  }

  private applyStatus(status: Status): void {
    this.auth.setAuthenticated(status.authenticated);
    this.state = status.state;
    this.connected = status.connected;
    this.mode = status.mode;
    this.lastError = status.last_error ?? null;
    this.since = status.since ?? null;
    if (status.location && !this.selectedServerId) {
      this.selectedServerId = status.location.id;
    }
  }

  async refreshStatus(): Promise<void> {
    if (!this.api.hasSessionToken()) return;
    try {
      const status = await this.api.status();
      runInAction(() => {
        this.applyStatus(status);
      });
      if (status.authenticated) {
        if (this.locations.length === 0) void this.loadLocations();
        if (status.connected) void this.loadProxy();
        else runInAction(() => { this.proxy = null; });
      }
    } catch {
      // поллинг — тихо игнорируем единичные сбои
    }
  }

  async loadLocations(): Promise<void> {
    try {
      const locations = await this.api.locations();
      runInAction(() => {
        this.locations = locations;
        if (!this.selectedServerId && locations.length > 0) {
          this.selectedServerId = locations[0].id;
        }
      });
    } catch {
      // некритично
    }
  }

  async loadProxy(): Promise<void> {
    try {
      const proxy = await this.api.proxy();
      runInAction(() => {
        this.proxy = proxy;
      });
    } catch {
      // некритично
    }
  }

  setSelectedServer(id: string): void {
    this.selectedServerId = id;
  }

  setSelectedMode(mode: ConnMode): void {
    this.selectedMode = mode;
  }

  async connect(): Promise<void> {
    this.busy = true;
    this.actionError = null;
    try {
      const res = await this.api.connect(
        this.selectedServerId ?? undefined,
        this.selectedMode,
      );
      runInAction(() => {
        this.state = res.state;
      });
      await this.refreshStatus();
    } catch (e) {
      runInAction(() => {
        this.actionError = e instanceof Error ? e.message : 'Connect failed';
      });
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }

  async disconnect(): Promise<void> {
    this.busy = true;
    this.actionError = null;
    try {
      const res = await this.api.disconnect();
      runInAction(() => {
        this.state = res.state;
        this.proxy = null;
      });
      await this.refreshStatus();
    } catch (e) {
      runInAction(() => {
        this.actionError = e instanceof Error ? e.message : 'Disconnect failed';
      });
    } finally {
      runInAction(() => {
        this.busy = false;
      });
    }
  }
}
