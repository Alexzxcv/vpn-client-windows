import { useEffect, useRef } from 'react';
import uPlot from 'uplot';
import 'uplot/dist/uPlot.min.css';

type OptionsFactory = (size: { width: number; height: number }) => uPlot.Options;

interface UPlotChartProps {
  data: uPlot.AlignedData;
  /** Build options from the measured container width and the given height. */
  options: OptionsFactory;
  height: number;
  className?: string;
}

/**
 * Thin uPlot wrapper: builds the chart, syncs data, resizes to container
 * width via ResizeObserver, and tears the instance down on unmount.
 */
export function UPlotChart({
  data,
  options,
  height,
  className,
}: UPlotChartProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const plotRef = useRef<uPlot | null>(null);
  const dataRef = useRef<uPlot.AlignedData>(data);
  const optionsRef = useRef<OptionsFactory>(options);

  dataRef.current = data;
  optionsRef.current = options;

  // Create / destroy the plot tied to the container element.
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const width = el.clientWidth || 300;
    const plot = new uPlot(
      optionsRef.current({ width, height }),
      dataRef.current,
      el,
    );
    plotRef.current = plot;

    const ro = new ResizeObserver((entries) => {
      const w = Math.floor(entries[0].contentRect.width);
      if (w > 0) plot.setSize({ width: w, height });
    });
    ro.observe(el);

    return () => {
      ro.disconnect();
      plot.destroy();
      plotRef.current = null;
    };
    // height is the only structural dep; data/options handled below
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [height]);

  // Push new data without recreating the instance.
  useEffect(() => {
    plotRef.current?.setData(data);
  }, [data]);

  return <div ref={containerRef} className={className} style={{ height }} />;
}
