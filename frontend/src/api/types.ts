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
  /** Web-dashboard URL (registration / account). Empty if not configured. */
  dashboard_url?: string;
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
  /** Control-plane→node RTT reported by the backend (0/absent if unknown).
   *  This is NOT the user's own ping; prefer `ping_ms` when present. */
  latency_ms?: number;
  /** The user's OWN measured ping (TCP RTT) to this node, in ms, measured by
   *  the local core. 0/absent when not yet measured or unreachable. */
  ping_ms?: number;
}

/** Sentinel server id for "Auto (best)": the backend picks the best node. */
export const AUTO_SERVER_ID = 'auto';

export interface Proxy {
  socks: string;
  http?: string;
}

/** User-supplied custom VLESS server (traffic not metered / not subscription-limited). */
export interface CustomServer {
  id: string;
  name: string;
  host: string;
  port: number;
}

/**
 * One local multi-proxy entry: a SOCKS5 listener on its own port pointed at a
 * specific server. `server_id` is 'auto' | a backend-location id | 'custom:<id>'.
 * `address` (e.g. "127.0.0.1:10810") is present once the proxy is running.
 */
export interface MultiProxyEntry {
  id: string;
  port: number;
  server_id: string;
  /** The "main" proxy receives the system proxy setting. */
  main: boolean;
  state: ConnState;
  error?: string;
  address?: string;
}

/** Free daily traffic allowance (bytes), mirrors backend.FreeDaily. */
export interface FreeDaily {
  limit_bytes: number;
  used_today_bytes: number;
  resets_at: string;
}

/** One traffic-over-time sample (cumulative used bytes at `ts`). */
export interface UsageSample {
  ts: string;
  used_bytes: number;
  limit_bytes: number;
}

/** Combined traffic snapshot from GET /api/usage (mirrors app.UsageInfo). */
export interface Usage {
  traffic_used_bytes: number;
  traffic_limit_bytes: number;
  free_daily?: FreeDaily;
  samples: UsageSample[];
}

export interface Me {
  id: string;
  email: string;
  is_admin: boolean;
}

export interface ConnectResult {
  state: ConnState;
}

/** Result of an update check against GitHub Releases (mirrors updater.Result). */
export interface UpdateResult {
  current_version: string;
  latest_version?: string;
  update_available: boolean;
  notes?: string;
  release_url?: string;
  asset_url?: string;
  asset_name?: string;
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
  /** Launch the client at Windows login (starts minimized to tray). */
  autostart?: boolean;
  /** Enable the multi-proxy feature: several local SOCKS5 proxies on
   *  different ports, each to its own server. Proxy-mode only. */
  multi_proxy_enabled?: boolean;
}
