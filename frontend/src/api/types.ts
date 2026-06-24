export type ConnState = 'disconnected' | 'connecting' | 'connected' | 'error';

/** Tunnelling mode. 'proxy' = local SOCKS/HTTP proxy via xray; 'tun' = full
 *  device-wide tunnel via sing-box (requires administrator rights). */
export type ConnMode = 'proxy' | 'tun';

export interface Bootstrap {
  session_token: string;
  api_base: string;
  version: string;
  /** Whether the core runs elevated (administrator). TUN mode needs this. */
  elevated: boolean;
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
