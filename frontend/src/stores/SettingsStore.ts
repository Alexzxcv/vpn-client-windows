import { makeAutoObservable, runInAction } from 'mobx';
import type { ControlApi } from '@/api/control';
import type { Settings } from '@/api/types';

/** Local client settings: ports, kill-switch, split-tunnel, Russia-direct. */
export class SettingsStore {
  private readonly api: ControlApi;

  current: Settings | null = null;
  loading = false;
  saving = false;
  error: string | null = null;
  saved = false;

  constructor(api: ControlApi) {
    this.api = api;
    makeAutoObservable(this, {}, { autoBind: true });
  }

  async load(): Promise<void> {
    if (!this.api.hasSessionToken()) return;
    this.loading = true;
    this.error = null;
    try {
      const s = await this.api.getSettings();
      runInAction(() => {
        this.current = s;
      });
    } catch (e) {
      runInAction(() => {
        this.error = e instanceof Error ? e.message : 'Failed to load settings';
      });
    } finally {
      runInAction(() => {
        this.loading = false;
      });
    }
  }

  async save(next: Settings): Promise<boolean> {
    this.saving = true;
    this.error = null;
    this.saved = false;
    try {
      const s = await this.api.saveSettings(next);
      runInAction(() => {
        this.current = s;
        this.saved = true;
      });
      return true;
    } catch (e) {
      runInAction(() => {
        this.error = e instanceof Error ? e.message : 'Failed to save settings';
      });
      return false;
    } finally {
      runInAction(() => {
        this.saving = false;
      });
    }
  }
}
