import uPlot from 'uplot';

/**
 * Dark uPlot presets — DESIGN_SYSTEM §5.
 * Colors are read from the live CSS palette so the chart tracks the tokens.
 */

function cssVar(name: string, fallback: string): string {
  if (typeof window === 'undefined') return fallback;
  const v = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
  return v || fallback;
}

const palette = () => ({
  ion: cssVar('--ion', '#3da9fc'),
  hairline: cssVar('--hairline', '#222835'),
  edge: cssVar('--edge', '#2e3542'),
  mute: cssVar('--mute', '#5a6273'),
});

/** Vertical ion gradient fill: rgba(61,169,252,0.18) -> 0. */
function ionFill(u: uPlot): CanvasGradient | string {
  const ctx = u.ctx;
  const { top, height } = u.bbox;
  const grad = ctx.createLinearGradient(0, top, 0, top + height);
  grad.addColorStop(0, 'rgba(61, 169, 252, 0.18)');
  grad.addColorStop(1, 'rgba(61, 169, 252, 0)');
  return grad;
}

type PresetOpts = {
  width: number;
  height: number;
  /** optional value formatter for the cursor legend / tooltip */
  fmt?: (v: number) => string;
};

const reducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

const baseAxes = (): uPlot.Axis[] => {
  const c = palette();
  const axis = {
    stroke: c.mute,
    grid: { stroke: c.hairline, width: 1 },
    ticks: { stroke: c.hairline, width: 1 },
    font: '10px "JetBrains Mono", monospace',
  };
  return [
    { ...axis, space: 48 },
    { ...axis, size: 36 },
  ];
};

const baseCursor = (): uPlot.Cursor => {
  // Cursor line color comes from the .u-cursor-x/.u-cursor-y CSS (edge tone).
  return {
    points: { size: 5, width: 1 },
    drag: { x: false, y: false },
    x: true,
    y: true,
  };
};

/** Mini line, no axes/grid — for compact metric cells. */
export function sparkline(opts: PresetOpts): uPlot.Options {
  const c = palette();
  return {
    width: opts.width,
    height: opts.height,
    padding: [4, 2, 2, 2],
    cursor: { show: false, x: false, y: false, drag: { x: false, y: false } },
    legend: { show: false },
    scales: { x: { time: false } },
    axes: [
      { show: false },
      { show: false },
    ],
    series: [
      {},
      {
        stroke: c.ion,
        width: 1.5,
        points: { show: false },
      },
    ],
  };
}

/** Mini area sparkline (ion gradient fill). */
export function areaSparkline(opts: PresetOpts): uPlot.Options {
  const c = palette();
  return {
    width: opts.width,
    height: opts.height,
    padding: [4, 2, 2, 2],
    cursor: { show: false, x: false, y: false, drag: { x: false, y: false } },
    legend: { show: false },
    scales: { x: { time: false } },
    axes: [
      { show: false },
      { show: false },
    ],
    series: [
      {},
      {
        stroke: c.ion,
        width: 1.5,
        fill: ionFill,
        points: { show: false },
      },
    ],
  };
}

/** Full line series with axes — latency over time. */
export function lineSeries(opts: PresetOpts): uPlot.Options {
  const c = palette();
  return {
    width: opts.width,
    height: opts.height,
    cursor: baseCursor(),
    legend: { show: false },
    scales: { x: { time: false } },
    axes: baseAxes(),
    series: [
      {},
      {
        stroke: c.ion,
        width: 1.5,
        points: { show: false },
        value: opts.fmt ? (_u, v) => (v == null ? '' : opts.fmt!(v)) : undefined,
      },
    ],
  };
}

/** Full area series with axes — traffic over time. */
export function areaSeries(opts: PresetOpts): uPlot.Options {
  const c = palette();
  return {
    width: opts.width,
    height: opts.height,
    cursor: baseCursor(),
    legend: { show: false },
    scales: { x: { time: false } },
    axes: baseAxes(),
    series: [
      {},
      {
        stroke: c.ion,
        width: 1.5,
        fill: ionFill,
        points: { show: false },
        value: opts.fmt ? (_u, v) => (v == null ? '' : opts.fmt!(v)) : undefined,
      },
    ],
  };
}

export const prefersReducedMotion = reducedMotion;
