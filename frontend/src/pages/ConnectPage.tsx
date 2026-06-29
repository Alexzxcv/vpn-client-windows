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
  Settings as SettingsIcon,
  ShieldAlert,
  Terminal,
  X,
} from 'lucide-react';
import { AUTO_SERVER_ID, type ConnMode } from '@/api/types';
import { useConnection, useAuth, useUpdate, useT } from '@/stores/context';
import { StatusBadge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, Eyebrow } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
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
  const t = useT();
  const navigate = useNavigate();
  const [copied, setCopied] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [customInput, setCustomInput] = useState('');
  const [adding, setAdding] = useState(false);

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
    { id: 'proxy', label: t('connect.modeProxy') },
    { id: 'tun', label: t('connect.modeTun') },
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

  // Copy the reconstructed vless:// link of a custom server to the clipboard.
  async function copyCustomLink(id: string) {
    const link = await conn.customServerLink(id);
    if (!link) return;
    try {
      await navigator.clipboard.writeText(link);
      setCopiedId(id);
      setTimeout(() => setCopiedId((cur) => (cur === id ? null : cur)), 1500);
    } catch {
      // clipboard unavailable — ignore
    }
  }

  const errorText =
    conn.actionError ?? (conn.state === 'error' ? conn.lastError : null);

  async function addCustom() {
    const value = customInput.trim();
    if (!value || adding) return;
    setAdding(true);
    try {
      const ok = await conn.addCustomServer(value);
      if (ok) setCustomInput('');
    } finally {
      setAdding(false);
    }
  }

  return (
    <TooltipProvider delayDuration={200}>
      <div className="flex h-full flex-col gap-3">
        {/* update banner — only when a newer release is published */}
        {update.available && (
          <div className="flex items-center gap-2 rounded-sm border border-ion/40 bg-ion/10 px-2.5 py-1.5 text-xs text-frost">
            <ArrowUpCircle className="h-4 w-4 shrink-0 text-ion" strokeWidth={1.5} />
            <span className="min-w-0 flex-1 break-words">
              {t('update.available', { version: update.latestVersion })}
              {update.error ? ` — ${update.error}` : ''}
            </span>
            <Button
              size="sm"
              onClick={() => void update.apply()}
              disabled={update.applying}
            >
              {update.applying ? t('update.downloading') : t('update.apply')}
            </Button>
            <button
              type="button"
              onClick={() => update.dismiss()}
              aria-label={t('update.dismiss')}
              className="text-mute hover:text-frost"
            >
              <X className="h-4 w-4" strokeWidth={1.5} />
            </button>
          </div>
        )}

        {/* header: status (left) · account + settings (right) */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
            <StatusBadge state={conn.state} />
            {conn.connected && (
              <span
                className={`font-mono text-2xs uppercase tracking-eyebrow ${
                  conn.mode === 'tun' ? 'text-ion' : 'text-ok'
                }`}
              >
                {conn.mode === 'tun'
                  ? t('connect.modeTun')
                  : t('connect.modeProxy')}
              </span>
            )}
          </div>
          <div className="flex min-w-0 items-center gap-1.5">
            {auth.me && (
              <span className="max-w-[150px] truncate font-mono text-2xs text-mute">
                {auth.me.email}
              </span>
            )}
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => navigate('/settings')}
                  aria-label={t('nav.settings')}
                >
                  <SettingsIcon className="h-4 w-4" strokeWidth={1.5} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('nav.settings')}</TooltipContent>
            </Tooltip>
          </div>
        </div>

        {/* signature RouteMap */}
        <Card className="bg-slate px-1 py-2">
          <RouteMap
            state={conn.state}
            toName={isAuto ? t('connect.autoName') : (selected?.name ?? '—')}
            toSub={isAuto ? t('connect.autoSub') : selected?.location}
            pingMs={pingMs > 0 ? pingMs : undefined}
          />
        </Card>

        {/* tunnelling mode toggle */}
        <div className="flex flex-col gap-1.5">
          <Eyebrow>{t('connect.mode')}</Eyebrow>
          <div
            role="radiogroup"
            aria-label={t('connect.mode')}
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
              <span className="break-words">{t('connect.tunNeedsAdmin')}</span>
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
          <Eyebrow>{t('connect.location')}</Eyebrow>
          <Select
            value={conn.selectedServerId}
            onValueChange={(v) => conn.setSelectedServer(v)}
            disabled={locked}
          >
            <SelectTrigger>
              <SelectValue placeholder={t('connect.locationLoading')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={AUTO_SERVER_ID}>{t('connect.auto')}</SelectItem>
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

        {/* custom (user-supplied) servers */}
        <Card className="flex flex-col gap-2 p-2.5">
          <Eyebrow>{t('custom.title')}</Eyebrow>

          <div className="flex items-center gap-1.5">
            <Input
              value={customInput}
              onChange={(e) => setCustomInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault();
                  void addCustom();
                }
              }}
              placeholder={t('custom.placeholder')}
              disabled={locked || adding}
              className="h-8 text-sm"
            />
            <Button
              size="sm"
              onClick={() => void addCustom()}
              disabled={locked || adding || customInput.trim() === ''}
            >
              {adding ? t('custom.adding') : t('common.add')}
            </Button>
          </div>

          {conn.customError && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-sm border border-alert/40 bg-alert/10 px-2.5 py-1.5 text-xs text-alert"
            >
              <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" strokeWidth={1.5} />
              <span className="break-words">{conn.customError}</span>
            </div>
          )}

          {conn.customServers.length > 0 && (
            <ul className="flex flex-col gap-1">
              {conn.customServers.map((s) => (
                <li
                  key={s.id}
                  className="flex items-center justify-between gap-2 rounded-sm bg-void px-2 py-1.5"
                >
                  <div className="flex min-w-0 flex-col">
                    <span className="truncate text-sm text-frost">{s.name}</span>
                    <code className="truncate font-mono text-2xs text-mute">
                      {s.host}:{s.port}
                    </code>
                  </div>
                  <div className="flex shrink-0 items-center">
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => void copyCustomLink(s.id)}
                      aria-label={t('custom.copyLink', { name: s.name })}
                      title={t('custom.copyLink', { name: s.name })}
                    >
                      {copiedId === s.id ? (
                        <Check className="h-4 w-4 text-ok" strokeWidth={1.5} />
                      ) : (
                        <Copy className="h-4 w-4" strokeWidth={1.5} />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => void conn.removeCustomServer(s.id)}
                      disabled={locked}
                      aria-label={t('custom.remove', { name: s.name })}
                      title={t('custom.remove', { name: s.name })}
                    >
                      <X className="h-4 w-4" strokeWidth={1.5} />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}

          <p className="text-2xs text-mute">{t('custom.note')}</p>
        </Card>

        {/* instrument cells */}
        <div className="grid grid-cols-2 gap-2">
          <MetricCell
            label={t('connect.ping')}
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
            label={t('connect.traffic')}
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
              <Eyebrow>{t('connect.socksProxy')}</Eyebrow>
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
                  aria-label={t('connect.copyCurlAria')}
                >
                  {copied ? (
                    <Check className="h-4 w-4 text-ok" strokeWidth={1.5} />
                  ) : (
                    <Copy className="h-4 w-4" strokeWidth={1.5} />
                  )}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                {copied ? t('common.copied') : t('connect.copyCurl')}
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

      </div>
    </TooltipProvider>
  );
});
