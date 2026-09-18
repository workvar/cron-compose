// SVG area chart for a simple time series (no chart library).
export type AreaPoint = { label: string; value: number };

export function AreaChart({ data }: { data: AreaPoint[] }) {
  const w = 320;
  const h = 120;
  const pad = 8;
  const max = Math.max(1, ...data.map((d) => d.value));
  const n = Math.max(1, data.length - 1);
  const pts = data.map((d, i) => {
    const x = pad + (i / n) * (w - pad * 2);
    const y = h - pad - (d.value / max) * (h - pad * 2);
    return `${x},${y}`;
  });
  const line = pts.join(" ");
  const area = `${pad},${h - pad} ${line} ${w - pad},${h - pad}`;

  return (
    <div className="chart-wrap">
      <svg className="chart-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Area chart">
        <polygon points={area} className="area-fill" />
        <polyline points={line} className="area-stroke" fill="none" />
      </svg>
      <div className="chart-xlabels">
        {data.map((d, i) => (
          <span key={i}>{d.label}</span>
        ))}
      </div>
    </div>
  );
}
