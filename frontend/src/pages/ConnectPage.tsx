import { observer } from 'mobx-react-lite';
import { useMemo, useState } from 'react';
import { AlertTriangle, Check, Copy, LogOut, Terminal } from 'lucide-react';
import { useConnection, useAuth } from '@/stores/context';
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
import { mockPingSeries, mockTrafficSeries } from '@/mock/metrics';

export const ConnectPage = observer(function ConnectPage() {
  const conn = useConnection();
  const auth = useAuth();
  const [copied, setCopied] = useState(false);

  const socks = conn.proxy?.socks ?? '127.0.0.1:10800';
  const curlHint = `curl --socks5 ${socks} https://ifconfig.me`;

  const selected = conn.locations.find((l) => l.id === conn.selectedServerId);
  const seedKey = conn.selectedServerId ?? 'default';

  // TODO: replace with API — mock metrics keyed off the selected node.
  const ping = useMemo(() => mockPingSeries(seedKey), [seedKey]);
  const traffic = useMemo(() => mockTrafficSeries(seedKey), [seedKey]);

  const locked = conn.state === 'connected' || conn.state === 'connecting';

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
        {/* status row */}
        <div className="flex items-center justify-between">
          <StatusBadge state={conn.state} />
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
            toName={selected?.name ?? '—'}
            toSub={selected?.location}
            pingMs={ping.last}
          />
        </Card>

        {/* connect control */}
        <ConnectButton
          state={conn.state}
          busy={conn.busy}
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
            value={conn.selectedServerId ?? undefined}
            onValueChange={(v) => conn.setSelectedServer(v)}
            disabled={locked}
          >
            <SelectTrigger>
              <SelectValue placeholder="Loading…" />
            </SelectTrigger>
            <SelectContent>
              {conn.locations.map((loc) => (
                <SelectItem key={loc.id} value={loc.id}>
                  {loc.name} — {loc.location}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </label>

        {/* instrument cells */}
        <div className="grid grid-cols-2 gap-2">
          <MetricCell
            label="Ping"
            value={String(ping.last)}
            unit="ms"
            chart={
              <UPlotChart
                data={ping.data}
                height={32}
                options={(s) => sparkline(s)}
              />
            }
          />
          <MetricCell
            label="Session"
            value={traffic.totalMb.toFixed(1)}
            unit="MB"
            chart={
              <UPlotChart
                data={traffic.data}
                height={32}
                options={(s) => areaSparkline(s)}
              />
            }
          />
        </div>

        {/* SOCKS proxy box */}
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

        {/* logout */}
        <div className="mt-auto flex justify-end pt-1">
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
