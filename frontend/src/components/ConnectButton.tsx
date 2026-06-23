import type { ConnState } from '@/api/types';

interface Props {
  state: ConnState;
  busy: boolean;
  onConnect: () => void;
  onDisconnect: () => void;
}

export function ConnectButton({ state, busy, onConnect, onDisconnect }: Props) {
  const isConnected = state === 'connected';
  const isConnecting = state === 'connecting';
  const disabled = busy || isConnecting;

  const label = isConnected
    ? 'Отключить'
    : isConnecting
      ? 'Подключение…'
      : 'Подключить';

  return (
    <button
      type="button"
      className={`connect-btn ${isConnected ? 'connect-btn-on' : 'connect-btn-off'}`}
      disabled={disabled}
      onClick={isConnected ? onDisconnect : onConnect}
    >
      <span className="connect-btn-icon">{isConnected ? '⏻' : '⏼'}</span>
      <span className="connect-btn-label">{label}</span>
    </button>
  );
}
