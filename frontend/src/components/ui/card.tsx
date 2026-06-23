import { cn } from '@/lib/utils';

/** Panel surface: slate bg + hairline border, sharp corners (DESIGN_SYSTEM §3). */
export function Card({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'rounded-md border border-hairline bg-slate',
        className,
      )}
      {...props}
    />
  );
}

/** Eyebrow label: mono, uppercase, wide tracking, muted (DESIGN_SYSTEM §2). */
export function Eyebrow({
  className,
  ...props
}: React.HTMLAttributes<HTMLSpanElement>) {
  return (
    <span
      className={cn(
        'font-mono text-2xs uppercase tracking-eyebrow text-mute',
        className,
      )}
      {...props}
    />
  );
}
