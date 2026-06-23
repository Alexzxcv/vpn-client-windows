import { api, type ControlApi } from '@/api/control';
import { AuthStore } from './AuthStore';
import { ConnectionStore } from './ConnectionStore';

export class RootStore {
  readonly api: ControlApi;
  readonly auth: AuthStore;
  readonly connection: ConnectionStore;

  constructor() {
    this.api = api;
    this.auth = new AuthStore(this.api);
    this.connection = new ConnectionStore(this.api, this.auth);
  }

  /** Инициализация на старте приложения. */
  async init(): Promise<void> {
    await this.auth.bootstrap();
    if (this.auth.bootstrapped) {
      this.connection.startPolling();
    }
  }
}
