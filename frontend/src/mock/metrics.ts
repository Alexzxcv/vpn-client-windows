import type uPlot from 'uplot';

// TODO: replace with API — real ping / session-traffic metrics will be
// streamed from the Go core later (see CONTROL_API.md). These deterministic
// generators only feed the sparkline visuals for now.

/** Small deterministic PRNG (mulberry32) so charts are stable per seed. */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const POINTS = 48;

/**
 * Ping sparkline (ms over the last ~POINTS samples), centered on a base value
 * that derives from the location id so each node looks distinct but stable.
 */
export function mockPingSeries(seedKey: string): {
  data: uPlot.AlignedData;
  last: number;
} {
  const seed = hash(seedKey);
  const rnd = mulberry32(seed);
  const base = 28 + (seed % 70); // 28..98 ms baseline
  const xs: number[] = [];
  const ys: number[] = [];
  for (let i = 0; i < POINTS; i++) {
    xs.push(i);
    const jitter = (rnd() - 0.5) * 18;
    const wobble = Math.sin(i / 5 + seed) * 6;
    ys.push(Math.max(8, Math.round(base + jitter + wobble)));
  }
  return { data: [xs, ys], last: ys[ys.length - 1] };
}

/**
 * Session throughput (KB/s) building up over the session — area sparkline.
 * Returns the series plus the integrated total (MB) for the value cell.
 */
export function mockTrafficSeries(seedKey: string): {
  data: uPlot.AlignedData;
  totalMb: number;
  lastKbps: number;
} {
  const seed = hash(seedKey) ^ 0x9e3779b9;
  const rnd = mulberry32(seed);
  const xs: number[] = [];
  const ys: number[] = [];
  let totalKb = 0;
  for (let i = 0; i < POINTS; i++) {
    xs.push(i);
    const ramp = Math.min(1, i / 10);
    const kbps = Math.round((120 + rnd() * 380) * ramp);
    ys.push(kbps);
    totalKb += kbps; // ~1 sample/sec
  }
  return {
    data: [xs, ys],
    totalMb: totalKb / 1024,
    lastKbps: ys[ys.length - 1],
  };
}

function hash(s: string): number {
  let h = 2166136261;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}
