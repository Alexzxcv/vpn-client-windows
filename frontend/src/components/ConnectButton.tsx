import { Loader2, Power } from 'lucide-react';
import type { ConnState } from '@/api/types';
import { cn } from '@/lib/utils';

interface Props {
  state: ConnState;
  busy: boolean;
  onConnect: () => void;
  onDisconnect: () => void;
}

const LABEL: Record<ConnState, string> = {
  disconnected: 'CONNECT',
  connecting: 'LINKING',
  connected: 'DISCONNECT',
  error: 'RETRY',
};

/** Strict round connect control with state coloring + connected glow. */
export function ConnectButton({ state, busy, onConnect, onDisconnect }: Props) {
  const isConnected = state === 'connected';
  const isConnecting = state === 'connecting';
  const disabled = busy || isConnecting;

  return (
    <button
      type="button"
      disabled={disabled}
      onClick={isConnected ? onDisconnect : onConnect}
      aria-label={LABEL[state]}
      className={cn(
        'group mx-auto flex h-[124px] w-[124px] flex-col items-center justify-center gap-1.5 rounded-full',
        'border-2 bg-graphite transition-[transform,border-color,box-shadow] duration-150',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ion focus-visible:ring-offset-2 focus-visible:ring-offset-void',
        'disabled:cursor-default disabled:opacity-80',
        !disabled && 'hover:scale-[1.03]',
        isConnected && 'border-ok text-ok shadow-ion-glow',
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
        {LABEL[state]}
      </span>
    </button>
  );
}
