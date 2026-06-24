import { observer } from 'mobx-react-lite';
import { useMemo, useState } from 'react';
import type uPlot from 'uplot';
import { useNavigate } from 'react-router-dom';
import {
  AlertTriangle,
  ArrowUpCircle,
  Check,
  Copy,
  Globe,
  LogOut,
  Settings as SettingsIcon,
  ShieldAlert,
  Terminal,
  X,
} from 'lucide-react';
import { AUTO_SERVER_ID, type ConnMode } from '@/api/types';
import { useConnection, useAuth, useUpdate } from '@/stores/context';
import { StatusBadge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, Eyebrow } from '@/components/ui/card';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { ConnectButton } from '@/components/ConnectButton';
import { MetricCell } from '@/components/MetricCell';
import { RouteMap } from '@/components/RouteMap';
import { UPlotChart } from '@/components/chart/UPlotChart';
import { sparkline, areaSparkline } from '@/components/chart/chartTheme';

/** Human-readable bytes (1024-based): "0 B", "12.3 MB", "1.4 GB". */
function formatBytes(bytes: number): { value: string; unit: string } {
  if (!Number.isFinite(bytes) || bytes <= 0) return { value: '0', unit: 'B' };
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let n = bytes;
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i += 1;
  }
  const value = i === 0 ? String(Math.round(n)) : n.toFixed(n < 10 ? 2 : 1);
  return { value, unit: units[i] };
}

export const ConnectPage = observer(function ConnectPage() {
  const conn = useConnection();
  const auth = useAuth();
  const update = useUpdate();
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);

  const socks = conn.proxy?.socks ?? '127.0.0.1:10800';
  const curlHint = `curl --socks5 ${socks} https://ifconfig.me`;

  const isAuto = conn.selectedServerId === AUTO_SERVER_ID;
  const selected = conn.locations.find((l) => l.id === conn.selectedServerId);

  // Ping: live latency for the selected node (Auto → lowest across nodes),
  // sparkline = the rolling window of real measurements from the store.
  const pingMs = conn.pingMs;
  const ping = useMemo<uPlot.AlignedData>(() => {
    const ys = conn.pingSamples;
    const xs = ys.map((_, i) => i);
    return [xs, ys];
  }, [conn.pingSamples]);

  // Traffic: cumulative used bytes over time from /api/usage; the headline
  // value is the current total used (free-daily today if present, else period).
  const usage = conn.usage;
  const traffic = useMemo<uPlot.AlignedData>(() => {
    const samples = usage?.samples ?? [];
    const xs = samples.map((_, i) => i);
    const ys = samples.map((s) => s.used_bytes);
    return [xs, ys];
  }, [usage]);
  const usedBytes = usage?.free_daily
    ? usage.free_daily.used_today_bytes
    : (usage?.traffic_used_bytes ?? 0);
  const trafficFmt = formatBytes(usedBytes);

  const locked = conn.state === 'connected' || conn.state === 'connecting';

  // TUN requires the core to run as administrator.
  const tunNeedsElevation = conn.selectedMode === 'tun' && !auth.elevated;

  const modes: { id: ConnMode; label: string }[] = [
    { id: 'proxy', label: 'Proxy' },
    { id: 'tun', label: 'Full tunnel' },
  ];

  async function copyCurl() {
    try {
      await navigator.clipboard.writeText(curlHint);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      // clipboard unavailable — ignore
    }
  }

  const errorText =
    conn.actionError ?? (conn.state === 'error' ? conn.lastError : null);

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex h-full flex-col gap-3">
        {/* update banner — only when a newer release is published */}
        {update.available && (
          <div className="flex items-center gap-2 rounded-sm border border-ion/40 bg-ion/10 px-2.5 py-1.5 text-xs text-frost">
            <ArrowUpCircle className="h-4 w-4 shrink-0 text-ion" strokeWidth={1.5} />
            <span className="min-w-0 flex-1 break-words">
              Доступно обновление {update.latestVersion}
              {update.error ? ` — ${update.error}` : ''}
            </span>
            <Button
              size="sm"
              onClick={() => void update.apply()}
              disabled={update.applying}
            >
              {update.applying ? 'Загрузка…' : 'Обновить'}
            </Button>
            <button
              type="button"
              onClick={() => update.dismiss()}
              aria-label="Dismiss update"
              className="text-mute hover:text-frost"
            >
              <X className="h-4 w-4" strokeWidth={1.5} />
            </button>
          </div>
        )}

        {/* status row */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <StatusBadge state={conn.state} />
            {conn.connected && (
              <span
                className={`font-mono text-2xs uppercase tracking-eyebrow ${
                  conn.mode === 'tun' ? 'text-ion' : 'text-ok'
                }`}
              >
                {conn.mode === 'tun' ? 'Full tunnel' : 'Proxy'}
              </span>
            )}
          </div>
          {auth.me && (
            <span className="max-w-[55%] truncate font-mono text-2xs text-mute">
              {auth.me.email}
            </span>
          )}
        </div>

        {/* signature RouteMap */}
        <Card className="bg-slate px-1 py-2">
          <RouteMap
            state={conn.state}
            toName={isAuto ? 'Auto (best)' : (selected?.name ?? '—')}
            toSub={isAuto ? 'lowest latency' : selected?.location}
            pingMs={pingMs > 0 ? pingMs : undefined}
          />
        </Card>

        {/* tunnelling mode toggle */}
        <div className="flex flex-col gap-1.5">
          <Eyebrow>Mode</Eyebrow>
          <div
            role="radiogroup"
            aria-label="Tunnelling mode"
            className="grid grid-cols-2 gap-1 rounded-sm border border-edge bg-void p-1"
          >
            {modes.map((m) => {
              const active = conn.selectedMode === m.id;
              const Icon = m.id === 'tun' ? Globe : Terminal;
              return (
                <button
                  key={m.id}
                  type="button"
                  role="radio"
                  aria-checked={active}
                  disabled={locked}
                  onClick={() => conn.setSelectedMode(m.id)}
                  className={`flex items-center justify-center gap-1.5 rounded-sm px-2 py-1.5 text-xs font-medium transition-colors disabled:pointer-events-none disabled:opacity-50 ${
                    active
                      ? 'bg-ion text-void'
                      : 'text-ash hover:bg-graphite hover:text-frost'
                  }`}
                >
                  <Icon className="h-3.5 w-3.5" strokeWidth={1.5} />
                  {m.label}
                </button>
              );
            })}
          </div>
          {tunNeedsElevation && (
            <div className="flex items-start gap-2 rounded-sm border border-warn/40 bg-warn/10 px-2.5 py-1.5 text-2xs text-warn">
              <ShieldAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" strokeWidth={1.5} />
              <span className="break-words">
                Полный туннель требует прав администратора. Запусти приложение от
                имени администратора.
              </span>
            </div>
          )}
        </div>

        {/* connect control */}
        <ConnectButton
          state={conn.state}
          mode={conn.mode}
          busy={conn.busy || (tunNeedsElevation && !conn.connected)}
          onConnect={() => void conn.connect()}
          onDisconnect={() => void conn.disconnect()}
        />

        {errorText && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-sm border border-alert/40 bg-alert/10 px-2.5 py-1.5 text-xs text-alert"
          >
            <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" strokeWidth={1.5} />
            <span className="break-words">{errorText}</span>
          </div>
        )}

        {/* location selector */}
        <label className="flex flex-col gap-1.5">
          <Eyebrow>Location</Eyebrow>
          <Select
            value={conn.selectedServerId}
            onValueChange={(v) => conn.setSelectedServer(v)}
            disabled={locked}
          >
            <SelectTrigger>
              <SelectValue placeholder="Loading…" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={AUTO_SERVER_ID}>
                Auto (best) — lowest latency
              </SelectItem>
              {conn.locations.map((loc) => {
                // Show the user's OWN measured ping, falling back to the
                // backend's control-plane latency when not yet measured.
                const ms = loc.ping_ms || loc.latency_ms || 0;
                return (
                  <SelectItem key={loc.id} value={loc.id}>
                    {loc.name} — {loc.location}
                    {ms ? ` · ${ms} ms` : ''}
                  </SelectItem>
                );
              })}
            </SelectContent>
          </Select>
        </label>

        {/* instrument cells */}
        <div className="grid grid-cols-2 gap-2">
          <MetricCell
            label="Ping"
            value={pingMs > 0 ? String(pingMs) : '—'}
            unit={pingMs > 0 ? 'ms' : ''}
            chart={
              <UPlotChart
                data={ping}
                height={32}
                options={(s) => sparkline(s)}
              />
            }
          />
          <MetricCell
            label="Traffic"
            value={trafficFmt.value}
            unit={trafficFmt.unit}
            chart={
              <UPlotChart
                data={traffic}
                height={32}
                options={(s) => areaSparkline(s)}
              />
            }
          />
        </div>

        {/* SOCKS proxy box — only relevant in proxy mode */}
        {conn.selectedMode === 'proxy' && (
        <Card className="flex flex-col gap-2 p-2.5">
          <div className="flex items-center justify-between gap-2">
            <div className="flex flex-col gap-0.5">
              <Eyebrow>SOCKS proxy</Eyebrow>
              <code
                className={`selectable font-mono text-sm tabnum ${
                  conn.connected ? 'text-ok' : 'text-ash'
                }`}
              >
                {socks}
              </code>
            </div>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="icon"
                  onClick={copyCurl}
                  aria-label="Copy curl test command"
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-ok" strokeWidth={1.5} />
                  ) : (
                    <Copy className="h-4 w-4" strokeWidth={1.5} />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {copied ? 'Copied' : 'Copy curl test'}
              </TooltipContent>
            </Tooltip>
          </div>
          <div className="flex items-start gap-1.5 rounded-sm bg-void px-2 py-1.5">
            <Terminal className="mt-0.5 h-3.5 w-3.5 shrink-0 text-mute" strokeWidth={1.5} />
            <code className="selectable break-all font-mono text-2xs text-mute">
              {curlHint}
            </code>
          </div>
        </Card>
        )}

        {/* footer actions */}
        <div className="mt-auto flex justify-end gap-1 pt-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => navigate('/settings')}
                aria-label="Settings"
              >
                <SettingsIcon className="h-4 w-4" strokeWidth={1.5} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Settings</TooltipContent>
          </Tooltip>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => void auth.logout()}
                aria-label="Log out"
              >
                <LogOut className="h-4 w-4" strokeWidth={1.5} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Log out</TooltipContent>
          </Tooltip>
        </div>
      </div>
    </TooltipProvider>
  );
});
