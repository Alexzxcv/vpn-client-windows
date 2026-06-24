import { api, type ControlApi } from '@/api/control';
import { AuthStore } from './AuthStore';
import { ConnectionStore } from './ConnectionStore';
import { SettingsStore } from './SettingsStore';

export class RootStore {
  readonly api: ControlApi;
  readonly auth: AuthStore;
  readonly connection: ConnectionStore;
  readonly settings: SettingsStore;

  constructor() {
    this.api = api;
    this.auth = new AuthStore(this.api);
    this.connection = new ConnectionStore(this.api, this.auth);
    this.settings = new SettingsStore(this.api);
  }

  /** Инициализация на старте приложения. */
  async init(): Promise<void> {
    await this.auth.bootstrap();
    if (this.auth.bootstrapped) {
      this.connection.startPolling();
    }
  }
}
