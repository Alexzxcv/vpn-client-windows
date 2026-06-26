import { observer } from 'mobx-react-lite';
import { Loader2, Power } from 'lucide-react';
import type { ConnMode, ConnState } from '@/api/types';
import { cn } from '@/lib/utils';
import { i18n } from '@/stores/I18nStore';

interface Props {
  state: ConnState;
  /** Active tunnelling mode — colours the connected state (proxy = green, tun = blue). */
  mode?: ConnMode;
  busy: boolean;
  onConnect: () => void;
  onDisconnect: () => void;
}

const LABEL_KEY: Record<ConnState, string> = {
  disconnected: 'status.connect',
  connecting: 'status.linking',
  connected: 'status.disconnect',
  error: 'status.retry',
};

/** Strict round connect control with state coloring + connected glow. */
export const ConnectButton = observer(function ConnectButton({
  state,
  mode,
  busy,
  onConnect,
  onDisconnect,
}: Props) {
  const isConnected = state === 'connected';
  const isConnecting = state === 'connecting';
  const disabled = busy || isConnecting;
  const tun = mode === 'tun';
  const label = i18n.t(LABEL_KEY[state]);

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={isConnected ? onDisconnect : onConnect}
      aria-label={label}
      className={cn(
        'group mx-auto flex h-[124px] w-[124px] flex-col items-center justify-center gap-1.5 rounded-full',
        'border-2 bg-graphite transition-[transform,border-color,box-shadow] duration-150',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ion focus-visible:ring-offset-2 focus-visible:ring-offset-void',
        'disabled:cursor-default disabled:opacity-80',
        !disabled && 'hover:scale-[1.03]',
        isConnected && 'shadow-ion-glow',
        isConnected && (tun ? 'border-ion text-ion' : 'border-ok text-ok'),
        isConnecting && 'border-warn text-warn',
        state === 'error' && 'border-alert text-alert',
        state === 'disconnected' && 'border-ion text-ion',
      )}
    >
      {isConnecting ? (
        <Loader2 className="h-8 w-8 animate-spin" strokeWidth={1.5} />
      ) : (
        <Power className="h-8 w-8" strokeWidth={1.5} />
      )}
      <span className="font-mono text-2xs uppercase tracking-eyebrow">
        {label}
      </span>
    </button>
  );
});
