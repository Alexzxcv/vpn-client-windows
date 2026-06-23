import type { ReactNode } from 'react';
import { Card, Eyebrow } from '@/components/ui/card';
import { cn } from '@/lib/utils';

interface MetricCellProps {
  label: string;
  value: string;
  unit?: string;
  /** sparkline chart node */
  chart?: ReactNode;
  className?: string;
}

/**
 * Instrument cell (DESIGN_SYSTEM §3): eyebrow label, large mono number,
 * sparkline beneath.
 */
export function MetricCell({
  label,
  value,
  unit,
  chart,
  className,
}: MetricCellProps) {
  return (
    <Card className={cn('flex flex-col gap-1.5 p-2.5', className)}>
      <Eyebrow>{label}</Eyebrow>
      <div className="flex items-baseline gap-1">
        <span className="font-mono text-lg font-medium tabnum text-frost">
          {value}
        </span>
        {unit && <span className="font-mono text-2xs text-mute">{unit}</span>}
      </div>
      {chart && <div className="-mx-0.5 mt-0.5">{chart}</div>}
    </Card>
  );
}
