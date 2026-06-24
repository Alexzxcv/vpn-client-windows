import { makeAutoObservable, runInAction } from 'mobx';
import type { ControlApi } from '@/api/control';
import type { UpdateResult } from '@/api/types';

/**
 * Tracks client auto-update availability (GitHub Releases). The check is cheap
 * and runs once after bootstrap; the banner is only shown when a newer version
 * is published. Applying an update is always user-initiated (the "Обновить"
 * button) — never automatic.
 */
export class UpdateStore {
  private readonly api: ControlApi;

  result: UpdateResult | null = null;
  checking = false;
  applying = false;
  error: string | null = null;
  /** Set once the user dismisses the banner for this session. */
  dismissed = false;

  constructor(api: ControlApi) {
    this.api = api;
    makeAutoObservable(this, {}, { autoBind: true });
  }

  get available(): boolean {
    return !!this.result?.update_available && !this.dismissed;
  }

  get latestVersion(): string {
    return this.result?.latest_version ?? '';
  }

  async check(): Promise<void> {
    if (!this.api.hasSessionToken()) return;
    this.checking = true;
    this.error = null;
    try {
      const res = await this.api.checkUpdate();
      runInAction(() => {
        this.result = res;
      });
    } catch (e) {
      runInAction(() => {
        this.error = e instanceof Error ? e.message : 'Update check failed';
      });
    } finally {
      runInAction(() => {
        this.checking = false;
      });
    }
  }

  /** Download + launch the installer. The core does not quit automatically. */
  async apply(): Promise<boolean> {
    this.applying = true;
    this.error = null;
    try {
      await this.api.applyUpdate();
      return true;
    } catch (e) {
      runInAction(() => {
        this.error = e instanceof Error ? e.message : 'Update failed';
      });
      return false;
    } finally {
      runInAction(() => {
        this.applying = false;
      });
    }
  }

  dismiss(): void {
    this.dismissed = true;
  }
}
