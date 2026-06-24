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

/** User-editable local client settings (mirrors internal/settings.Settings). */
export interface Settings {
  socks_port: number;
  http_port: number;
  /** Block all egress if the tunnel drops. Defaults on for TUN. */
  kill_switch?: boolean;
  /** Manual split-tunnel direct list: domains (".ru") and/or IP CIDRs. */
  direct_list?: string[];
  /** Route Russian sites/IPs directly via geosite:ru / geoip:ru. */
  russia_direct?: boolean;
}
