import { cva, type VariantProps } from 'class-variance-authority';
import { observer } from 'mobx-react-lite';
import { cn } from '@/lib/utils';
import type { ConnState } from '@/api/types';
import { i18n } from '@/stores/I18nStore';

const dotVariants = cva('inline-block h-2 w-2 rounded-full shrink-0', {
  variants: {
    tone: {
      ok: 'bg-ok',
      warn: 'bg-warn',
      alert: 'bg-alert',
      idle: 'bg-mute',
    },
  },
  defaultVariants: { tone: 'idle' },
});

export type StatusTone = NonNullable<
  VariantProps<typeof dotVariants>['tone']
>;

/** Small status dot (DESIGN_SYSTEM §4). Optional pulse for connecting. */
export function StatusDot({
  tone,
  pulse = false,
  className,
}: {
  tone: StatusTone;
  pulse?: boolean;
  className?: string;
}) {
  return (
    <span className={cn('relative inline-flex', className)}>
      <span className={dotVariants({ tone })} />
      {pulse && (
        <span
          className={cn(
            'absolute inset-0 animate-ping rounded-full opacity-60',
            dotVariants({ tone }),
          )}
        />
      )}
    </span>
  );
}

const STATE_TONE: Record<ConnState, StatusTone> = {
  disconnected: 'idle',
  connecting: 'warn',
  connected: 'ok',
  error: 'alert',
};

const STATE_LABEL_KEY: Record<ConnState, string> = {
  disconnected: 'status.offline',
  connecting: 'status.linking',
  connected: 'status.secured',
  error: 'status.error',
};

/** Eyebrow-style status badge: dot + uppercase mono label. */
export const StatusBadge = observer(function StatusBadge({
  state,
  className,
}: {
  state: ConnState;
  className?: string;
}) {
  const tone = STATE_TONE[state];
  return (
    <span
      className={cn(
        'inline-flex items-center gap-2 font-mono text-2xs uppercase tracking-eyebrow',
        state === 'connected' && 'text-ok',
        state === 'connecting' && 'text-warn',
        state === 'error' && 'text-alert',
        state === 'disconnected' && 'text-mute',
        className,
      )}
    >
      <StatusDot tone={tone} pulse={state === 'connecting'} />
      {i18n.t(STATE_LABEL_KEY[state])}
    </span>
  );
});
