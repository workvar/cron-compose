// Scatter of run duration vs start time.
export type ScatterPoint = { x: number; y: number; label?: string; tone?: "ok" | "danger" | "neutral" };

export function ScatterPlot({ points }: { points: ScatterPoint[] }) {
  const w = 320;
  const h = 140;
  const pad = 16;
  const maxX = Math.max(1, ...points.map((p) => p.x));
  const maxY = Math.max(1, ...points.map((p) => p.y));

  return (
    <svg className="chart-svg" viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Scatter plot">
      <line x1={pad} y1={h - pad} x2={w - pad} y2={h - pad} className="chart-axis" />
      <line x1={pad} y1={pad} x2={pad} y2={h - pad} className="chart-axis" />
      {points.map((p, i) => {
        const cx = pad + (p.x / maxX) * (w - pad * 2);
        const cy = h - pad - (p.y / maxY) * (h - pad * 2);
        return (
          <circle
            key={i}
            cx={cx}
            cy={cy}
            r={4}
            className={`scatter-dot ${p.tone ?? "neutral"}`}
          >
            {p.label ? <title>{p.label}</title> : null}
          </circle>
        );
      })}
    </svg>
  );
}
