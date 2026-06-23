export type ConnState = 'disconnected' | 'connecting' | 'connected' | 'error';

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
