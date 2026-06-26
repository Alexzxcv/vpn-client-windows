import { api, type ControlApi } from '@/api/control';
import { AuthStore } from './AuthStore';
import { ConnectionStore } from './ConnectionStore';
import { SettingsStore } from './SettingsStore';
import { UpdateStore } from './UpdateStore';
import { i18n } from './I18nStore';

export class RootStore {
  readonly api: ControlApi;
  readonly auth: AuthStore;
  readonly connection: ConnectionStore;
  readonly settings: SettingsStore;
  readonly update: UpdateStore;
  readonly i18n = i18n;

  constructor() {
    this.api = api;
    this.auth = new AuthStore(this.api);
    this.connection = new ConnectionStore(this.api, this.auth);
    this.settings = new SettingsStore(this.api);
    this.update = new UpdateStore(this.api);
  }

  /** Инициализация на старте приложения. */
  async init(): Promise<void> {
    await this.auth.bootstrap();
    if (this.auth.bootstrapped) {
      this.connection.startPolling();
      // Surface a newer release if one is published (non-blocking).
      void this.update.check();
    }
  }
}
