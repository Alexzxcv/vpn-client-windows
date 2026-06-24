import { useEffect, useRef, useState } from 'react';
import type { ConnState } from '@/api/types';
import { cn } from '@/lib/utils';

interface RouteMapProps {
  state: ConnState;
  /** Origin label, e.g. "YOU". */
  fromLabel?: string;
  /** Destination node name (city). */
  toName: string;
  /** Destination sub-label (country / region). */
  toSub?: string;
  /** Latency in ms shown at the node end. */
  pingMs?: number;
  className?: string;
}

const W = 360;
const H = 132;
const PAD = 28;
const Y = H / 2;
const X0 = PAD;
const X1 = W - PAD;

/**
 * Signature element (DESIGN_SYSTEM §6): YOU -> NODE.
 * disconnected -> dashed static line; connecting/connected -> moving packet.
 * Respects prefers-reduced-motion.
 */
export function RouteMap({
  state,
  fromLabel = 'YOU',
  toName,
  toSub,
  pingMs,
  className,
}: RouteMapProps) {
  const packetRef = useRef<SVGCircleElement>(null);
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    const apply = () => setReduced(mq.matches);
    apply();
    mq.addEventListener('change', apply);
    return () => mq.removeEventListener('change', apply);
  }, []);

  const animate = state === 'connecting' || state === 'connected';

  // Right-anchored labels near the destination node never overflow the right
  // edge; long node names are ellipsised so they don't drive off the graph.
  const clip = (s: string, n: number) =>
    s.length > n ? `${s.slice(0, n - 1)}…` : s;
  const labelX = X1 + 20;

  // requestAnimationFrame-driven packet travelling along the line.
  useEffect(() => {
    const packet = packetRef.current;
    if (!packet) return;
    if (!animate || reduced) {
      // park the packet at the origin when idle / reduced motion
      packet.setAttribute('cx', String(X0));
      packet.style.opacity = animate && reduced ? '1' : '0';
      if (animate && reduced) packet.setAttribute('cx', String((X0 + X1) / 2));
      return;
    }
    let raf = 0;
    const speed = state === 'connected' ? 0.00065 : 0.0012; // px-frac per ms
    const start = performance.now();
    const tick = (now: number) => {
      const t = ((now - start) * speed) % 1;
      const cx = X0 + (X1 - X0) * t;
      packet.setAttribute('cx', String(cx));
      packet.style.opacity = String(0.35 + 0.65 * Math.sin(Math.PI * t));
      raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [animate, reduced, state]);

  const lineColor =
    state === 'error'
      ? 'var(--alert)'
      : state === 'disconnected'
        ? 'var(--edge)'
        : 'var(--ion)';
  const dashed = state === 'disconnected' || state === 'error';

  return (
    <svg
      viewBox={`0 0 ${W} ${H}`}
      width="100%"
      className={cn('block', className)}
      role="img"
      aria-label={`Route from ${fromLabel} to ${toName}${
        pingMs != null ? `, ${pingMs} milliseconds` : ''
      }, status ${state}`}
    >
      <defs>
        <radialGradient id="rm-node-glow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stopColor="var(--ion)" stopOpacity="0.35" />
          <stop offset="100%" stopColor="var(--ion)" stopOpacity="0" />
        </radialGradient>
      </defs>

      {/* connection line */}
      <line
        x1={X0}
        y1={Y}
        x2={X1}
        y2={Y}
        stroke={lineColor}
        strokeWidth={1.5}
        strokeDasharray={dashed ? '4 5' : undefined}
        strokeLinecap="round"
      />

      {/* travelling packet */}
      <circle
        ref={packetRef}
        cx={X0}
        cy={Y}
        r={3.5}
        fill="var(--ion)"
        style={{ opacity: 0 }}
      />

      {/* origin node */}
      <circle cx={X0} cy={Y} r={5} fill="var(--graphite)" stroke="var(--ash)" strokeWidth={1.5} />
      <text
        x={X0}
        y={Y - 16}
        textAnchor="middle"
        className="fill-[var(--mute)] font-mono"
        fontSize="9"
        letterSpacing="0.12em"
      >
        {fromLabel}
      </text>

      {/* destination node */}
      {state === 'connected' && (
        <circle cx={X1} cy={Y} r={16} fill="url(#rm-node-glow)" />
      )}
      <circle
        cx={X1}
        cy={Y}
        r={6}
        fill={state === 'connected' ? 'var(--ion)' : 'var(--graphite)'}
        stroke={state === 'connected' ? 'var(--ion)' : 'var(--edge)'}
        strokeWidth={1.5}
      />
      <text
        x={labelX}
        y={Y - 16}
        textAnchor="end"
        className="fill-[var(--frost)] font-mono"
        fontSize="11"
      >
        {clip(toName, 22)}
      </text>
      {toSub && (
        <text
          x={labelX}
          y={Y + 22}
          textAnchor="end"
          className="fill-[var(--mute)] font-mono"
          fontSize="9"
          letterSpacing="0.08em"
        >
          {clip(toSub.toUpperCase(), 24)}
        </text>
      )}
      {pingMs != null && state === 'connected' && (
        <text
          x={labelX}
          y={Y + 34}
          textAnchor="end"
          className="fill-[var(--ok)] font-mono"
          fontSize="9"
        >
          {pingMs} ms
        </text>
      )}
    </svg>
  );
}
