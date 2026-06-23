export type ConnState = 'disconnected' | 'connecting' | 'connected' | 'error';

/** Tunnelling mode. Currently always 'proxy'; 'tun' is reserved for a future
 *  full WinTUN tunnel. */
export type ConnMode = 'proxy' | 'tun';

export interface Bootstrap {
  session_token: string;
  api_base: string;
  version: string;
}

export interface StatusLocation {
  id: string;
  name: string;
}

export interface Status {
  authenticated: boolean;
  connected: boolean;
  state: ConnState;
  mode: ConnMode;
  proxy_address?: string;
  location?: StatusLocation;
  since?: string;
  last_error?: string;
}

export interface Location {
  id: string;
  name: string;
  location: string;
}

export interface Proxy {
  socks: string;
  http?: string;
}

export interface Me {
  id: string;
  email: string;
  is_admin: boolean;
}

export interface ConnectResult {
  state: ConnState;
}
