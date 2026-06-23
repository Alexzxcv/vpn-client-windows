import type { ConnState } from '@/api/types';

const LABELS: Record<ConnState, string> = {
  disconnected: 'Отключено',
  connecting: 'Подключение…',
  connected: 'Подключено',
  error: 'Ошибка',
};

export function StatusBadge({ state }: { state: ConnState }) {
  return (
    <span className={`badge badge-${state}`}>
      <span className="badge-dot" />
      {LABELS[state]}
    </span>
  );
}
