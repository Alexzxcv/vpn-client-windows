import { makeAutoObservable, runInAction } from 'mobx';
import type { ControlApi } from '@/api/control';
import {
  AUTO_SERVER_ID,
  type ConnMode,
  type ConnState,
  type Location,
  type Proxy,
  type Status,
  type Usage,
} from '@/api/types';
import type { AuthStore } from './AuthStore';

const POLL_INTERVAL_MS = 2000;
/** How often we re-measure latency (locations) and refresh traffic usage. */
const METRICS_INTERVAL_MS = 5000;
/** Rolling ping window size for the sparkline. */
const PING_WINDOW = 48;

export class ConnectionStore {
  private readonly api: ControlApi;
  private readonly auth: AuthStore;
  private pollTimer: ReturnType<typeof setInterval> | null = null;
  private metricsTimer: ReturnType<typeof setInterval> | null = null;

  state: ConnState = 'disconnected';
  connected = false;
  lastError: string | null = null;
  since: string | null = null;

  /** Actual active mode reported by the core. */
  mode: ConnMode = 'proxy';
  /** Mode the user picked for the next connect. */
  selectedMode: ConnMode = 'proxy';

  locations: Location[] = [];
  /** Defaults to "Auto (best)": the backend picks the lowest-latency node. */
  selectedServerId: string = AUTO_SERVER_ID;
  proxy: Proxy | null = null;

  busy = false;
  actionError: string | null = null;

  /** Rolling window of measured latency (ms) for the selected node, newest last. */
  pingSamples: number[] = [];
  /** Latest traffic usage snapshot (totals + samples) from /api/usage. */
  usage: Usage | null = null;

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

    void this.refreshMetrics();
    this.metricsTimer = setInterval(() => {
      void this.refreshMetrics();
    }, METRICS_INTERVAL_MS);
  }

  stopPolling(): void {
    if (this.pollTimer !== null) {
      clearInterval(this.pollTimer);
      this.pollTimer = null;
    }
    if (this.metricsTimer !== null) {
      clearInterval(this.metricsTimer);
      this.metricsTimer = null;
    }
  }

  /**
   * Re-measure latency (via locations) and refresh traffic usage. Driven on a
   * slower cadence than status; silent on transient failures so the sparklines
   * keep their last good shape.
   */
  async refreshMetrics(): Promise<void> {
    if (!this.api.hasSessionToken() || !this.auth.authenticated) return;
    await this.loadLocations();
    this.recordPingSample();
    await this.loadUsage();
  }

  /**
   * Effective ping (ms) for a location: the user's OWN measured ping (`ping_ms`),
   * falling back to the backend's control-plane latency (`latency_ms`) when the
   * client has not measured it yet. 0 if neither is known.
   */
  private effectivePing(loc: Location): number {
    const measured = loc.ping_ms ?? 0;
    if (measured > 0) return measured;
    return loc.latency_ms ?? 0;
  }

  /**
   * Ping (ms) for the current selection: the selected node's measured ping, or
   * — for "Auto (best)" — the minimum measured ping across all nodes. 0 if
   * unknown. Uses the user's real measurement (falling back to backend latency).
   */
  get pingMs(): number {
    if (this.selectedServerId === AUTO_SERVER_ID) {
      const vals = this.locations
        .map((l) => this.effectivePing(l))
        .filter((v) => v > 0);
      return vals.length > 0 ? Math.min(...vals) : 0;
    }
    const sel = this.locations.find((l) => l.id === this.selectedServerId);
    return sel ? this.effectivePing(sel) : 0;
  }

  /** Append the current latency to the rolling window (only when measured). */
  private recordPingSample(): void {
    const v = this.pingMs;
    if (v <= 0) return;
    runInAction(() => {
      this.pingSamples = [...this.pingSamples, v].slice(-PING_WINDOW);
    });
  }

  async loadUsage(): Promise<void> {
    try {
      const usage = await this.api.usage(24);
      runInAction(() => {
        this.usage = usage;
      });
    } catch {
      // некритично — оставляем последнее значение
    }
  }

  private applyStatus(status: Status): void {
    this.auth.setAuthenticated(status.authenticated);
    this.state = status.state;
    this.connected = status.connected;
    this.mode = status.mode;
    this.lastError = status.last_error ?? null;
    this.since = status.since ?? null;
    // Do NOT override the user's selection (e.g. "Auto (best)") with the
    // backend-resolved node id from the live status.
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
        // Keep the default "Auto (best)" selection; the user opts into a
        // specific node explicitly. Only fall back to a concrete node if the
        // current selection no longer exists (and is not the auto sentinel).
        if (
          this.selectedServerId !== AUTO_SERVER_ID &&
          !locations.some((l) => l.id === this.selectedServerId) &&
          locations.length > 0
        ) {
          this.selectedServerId = AUTO_SERVER_ID;
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
    if (id !== this.selectedServerId) {
      // The ping window tracks the selected node; reset it on a switch and seed
      // it with the new node's current measurement.
      this.pingSamples = [];
    }
    this.selectedServerId = id;
    this.recordPingSample();
  }

  setSelectedMode(mode: ConnMode): void {
    this.selectedMode = mode;
  }

  async connect(): Promise<void> {
    this.busy = true;
    this.actionError = null;
    try {
      // "auto" is sent through verbatim; the core treats it as "no server_id"
      // so the backend picks the best node.
      const res = await this.api.connect(
        this.selectedServerId,
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
