// Radar / spider chart for a small set of normalized axes (0–100).
export type RadarAxis = { label: string; value: number };

export function RadarChart({ axes }: { axes: RadarAxis[] }) {
  const n = axes.length;
  const size = 180;
  const cx = size / 2;
  const cy = size / 2;
  const radius = 68;

  function point(i: number, value: number) {
    const angle = (-Math.PI / 2) + (i / n) * Math.PI * 2;
    const r = (Math.max(0, Math.min(100, value)) / 100) * radius;
    return [cx + Math.cos(angle) * r, cy + Math.sin(angle) * r] as const;
  }

  const poly = axes.map((a, i) => point(i, a.value).join(",")).join(" ");
  const rings = [25, 50, 75, 100];

  return (
    <svg className="chart-svg radar" viewBox={`0 0 ${size} ${size}`} role="img" aria-label="Radar chart">
      {rings.map((pct) => (
        <polygon
          key={pct}
          className="radar-ring"
          points={axes.map((_, i) => point(i, pct).join(",")).join(" ")}
        />
      ))}
      {axes.map((a, i) => {
        const [x, y] = point(i, 100);
        return <line key={i} x1={cx} y1={cy} x2={x} y2={y} className="radar-spoke" />;
      })}
      <polygon points={poly} className="radar-fill" />
      {axes.map((a, i) => {
        const [x, y] = point(i, 112);
        return (
          <text key={a.label} x={x} y={y} textAnchor="middle" className="radar-label">{a.label}</text>
        );
      })}
    </svg>
  );
}
